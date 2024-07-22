package statsig

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	StatsigAPI = "https://statsigapi.net/v1"
	StatsigCDN = "https://api.statsigcdn.com/v1"
)

const (
	maxRetries        = 5
	backoffMultiplier = 10
)

type apis struct {
	downloadConfigSpecs string
	getIDLists          string
	logEvent            string
}

type transport struct {
	api      apis
	sdkKey   string
	metadata statsigMetadata // Safe to read from but not thread safe to write into. If value needs to change, please ensure thread safety.
	client   *http.Client
	options  *Options
}

func newTransport(secret string, options *Options) *transport {
	api := apis{
		downloadConfigSpecs: strings.TrimSuffix(defaultString(
			options.APIOverrides.DownloadConfigSpecs,
			defaultString(options.API, StatsigCDN),
		), "/"),
		getIDLists: strings.TrimSuffix(defaultString(
			options.APIOverrides.GetIDLists,
			defaultString(options.API, StatsigAPI),
		), "/"),
		logEvent: strings.TrimSuffix(defaultString(
			options.APIOverrides.LogEvent,
			defaultString(options.API, StatsigAPI),
		), "/"),
	}
	defer func() {
		if err := recover(); err != nil {
			Logger().LogError(err)
		}
	}()

	return &transport{
		api:      api,
		metadata: getStatsigMetadata(),
		sdkKey:   secret,
		client:   &http.Client{Timeout: time.Second * 3},
		options:  options,
	}
}

type RequestOptions struct {
	retries int
	backoff time.Duration
	header  map[string]string
}

func (opts *RequestOptions) fill_defaults() {
	if opts.backoff == 0 {
		opts.backoff = time.Second
	}
}

func (transport *transport) download_config_specs(sinceTime int64, responseBody interface{}) (*http.Response, error) {
	var endpoint string
	if transport.options.DisableCDN {
		endpoint = fmt.Sprintf("/download_config_specs?sinceTime=%d", sinceTime)
	} else {
		endpoint = fmt.Sprintf("/download_config_specs/%s.json?sinceTime=%d", transport.sdkKey, sinceTime)
	}
	return transport.get(endpoint, responseBody, RequestOptions{})
}

func (transport *transport) get_id_lists(responseBody interface{}) (*http.Response, error) {
	return transport.post("/get_id_lists", nil, responseBody, RequestOptions{})
}

func (transport *transport) get_id_list(url string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, &TransportError{Err: err}
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	res, err := transport.client.Do(req)

	if err != nil {
		var statusCode int
		if res != nil {
			statusCode = res.StatusCode
		}
		return res, &TransportError{
			RequestMetadata: &RequestMetadata{
				StatusCode: statusCode,
				Endpoint:   url,
				Retries:    0,
			},
			Err: err}
	}

	return res, nil
}

func (transport *transport) log_event(event []interface{}, responseBody interface{}, options RequestOptions) (*http.Response, error) {
	input := logEventInput{
		Events:          event,
		StatsigMetadata: transport.metadata,
	}
	if options.header == nil {
		options.header = make(map[string]string)
	}
	options.header["statsig-event-count"] = strconv.Itoa(len(event))
	return transport.post("/log_event", input, responseBody, options)

}

func (transport *transport) post(endpoint string, body interface{}, responseBody interface{}, options RequestOptions) (*http.Response, error) {
	return transport.doRequest("POST", endpoint, body, responseBody, options)
}

func (transport *transport) get(endpoint string, responseBody interface{}, options RequestOptions) (*http.Response, error) {
	return transport.doRequest("GET", endpoint, nil, responseBody, options)
}

func (transport *transport) buildRequest(method, endpoint string, body interface{}, header map[string]string) (*http.Request, error) {
	if transport.options.LocalMode {
		return nil, nil
	}

	var bodyBuf io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyBuf = bytes.NewBuffer(bodyBytes)

		if strings.Contains(endpoint, "log_event") {
			var compressedBody bytes.Buffer
			gz := gzip.NewWriter(&compressedBody)
			_, _ = gz.Write(bodyBytes)
			gz.Close()
			bodyBuf = &compressedBody
		}
	} else {
		if method == "POST" {
			bodyBuf = bytes.NewBufferString("{}")
		}
	}
	req, err := http.NewRequest(method, transport.buildURL(endpoint), bodyBuf)
	if err != nil {
		return nil, err
	}

	req.Header.Add("STATSIG-API-KEY", transport.sdkKey)
	req.Header.Set("Content-Type", "application/json")
	if strings.Contains(endpoint, "log_event") {
		req.Header.Set("Content-Encoding", "gzip")
	}
	req.Header.Add("STATSIG-CLIENT-TIME", strconv.FormatInt(getUnixMilli(), 10))
	req.Header.Add("STATSIG-SERVER-SESSION-ID", transport.metadata.SessionID)
	req.Header.Add("STATSIG-SDK-TYPE", transport.metadata.SDKType)
	req.Header.Add("STATSIG-SDK-VERSION", transport.metadata.SDKVersion)
	req.Header.Add("STATSIG-SDK-LANGUAGE-VERSION", transport.metadata.LanguageVersion)
	for k, v := range header {
		req.Header.Add(k, v)

	}
	return req, nil
}

func (transport *transport) buildURL(endpoint string) string {
	if strings.Contains(endpoint, "download_config_specs") {
		return transport.api.downloadConfigSpecs + endpoint
	} else if strings.Contains(endpoint, "get_id_list") {
		return transport.api.getIDLists + endpoint
	} else if strings.Contains(endpoint, "log_event") {
		return transport.api.logEvent + endpoint
	} else {
		return defaultString(transport.options.API, StatsigAPI) + endpoint
	}
}

func (transport *transport) doRequest(
	method string,
	endpoint string,
	in interface{},
	out interface{},
	options RequestOptions,
) (*http.Response, error) {
	request, err := transport.buildRequest(method, endpoint, in, options.header)
	if request == nil || err != nil {
		if err != nil {
			return nil, &TransportError{Err: err}
		}
		return nil, nil
	}
	options.fill_defaults()
	response, err, attempts := retry(options.retries, time.Duration(options.backoff), func() (*http.Response, bool, error) {
		response, err := transport.client.Do(request)
		if err != nil {
			return response, response != nil, err
		}
		drainAndCloseBody := func() {
			if response.Body != nil {
				// Drain body to re-use the same connection
				_, _ = io.Copy(ioutil.Discard, response.Body)
				response.Body.Close()
			}
		}
		defer drainAndCloseBody()

		if response.StatusCode >= 200 && response.StatusCode < 300 {
			return response, false, transport.parseResponse(response, out)
		}

		return response, retryableStatusCode(response.StatusCode), fmt.Errorf(response.Status)
	})

	if err != nil {
		if response == nil {
			return response, &TransportError{Err: err}
		}
		return response, &TransportError{
			RequestMetadata: &RequestMetadata{
				StatusCode: response.StatusCode,
				Endpoint:   endpoint,
				Retries:    attempts,
			},
			Err: err,
		}
	}

	return response, nil
}

func (transport *transport) parseResponse(response *http.Response, out interface{}) error {
	if out == nil {
		return nil
	}
	return json.NewDecoder(response.Body).Decode(&out)
}

func retry(retries int, backoff time.Duration, fn func() (*http.Response, bool, error)) (*http.Response, error, int) {
	attempts := 0
	for {
		if response, retry, err := fn(); retry {
			if retries <= 0 {
				return response, err, attempts
			}

			retries--
			attempts++
			time.Sleep(backoff)
			backoff = backoff * backoffMultiplier
		} else {
			return response, err, attempts
		}
	}
}

func retryableStatusCode(code int) bool {
	switch code {
	case 408, 500, 502, 503, 504, 522, 524, 599:
		return true
	default:
		return false
	}
}
