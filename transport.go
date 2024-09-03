package statsig

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

type transport struct {
	sdkKey   string
	metadata statsigMetadata // Safe to read from but not thread safe to write into. If value needs to change, please ensure thread safety.
	client   *http.Client
	options  *Options
}

func newTransport(secret string, options *Options) *transport {
	defer func() {
		if err := recover(); err != nil {
			Logger().LogError(err)
		}
	}()

	return &transport{
		metadata: getStatsigMetadata(),
		sdkKey:   secret,
		client: &http.Client{
			Timeout:   time.Second * 3,
			Transport: options.Transport,
		},
		options: options,
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

func (transport *transport) download_config_specs(sinceTime int64, responseBody interface{}, diagnostics *marker) (*http.Response, error) {
	diagnostics.downloadConfigSpecs().networkRequest().start().mark()
	var endpoint string
	if transport.options.DisableCDN {
		endpoint = fmt.Sprintf("/download_config_specs?sinceTime=%d", sinceTime)
	} else {
		endpoint = fmt.Sprintf("/download_config_specs/%s.json?sinceTime=%d", transport.sdkKey, sinceTime)
	}
	options := RequestOptions{}
	if transport.options.FallbackToStatsigAPI {
		options.retries = 1
	}
	return transport.get(endpoint, responseBody, options, diagnostics)
}

func (transport *transport) get_id_lists(responseBody interface{}, diagnostics *marker) (*http.Response, error) {
	diagnostics.getIdListSources().networkRequest().start().mark()
	options := RequestOptions{}
	if transport.options.FallbackToStatsigAPI {
		options.retries = 1
	}
	return transport.post("/get_id_lists", nil, responseBody, options, diagnostics)
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

func (transport *transport) log_event(
	event []interface{},
	responseBody interface{},
	options RequestOptions,
) (*http.Response, error) {
	input := logEventInput{
		Events:          event,
		StatsigMetadata: transport.metadata,
	}
	if options.header == nil {
		options.header = make(map[string]string)
	}
	options.header["statsig-event-count"] = strconv.Itoa(len(event))
	return transport.post("/log_event", input, responseBody, options, nil)

}

func (transport *transport) post(
	endpoint string,
	body interface{},
	responseBody interface{},
	options RequestOptions,
	diagnostics *marker,
) (*http.Response, error) {
	return transport.doRequest("POST", endpoint, body, responseBody, options, diagnostics)
}

func (transport *transport) get(
	endpoint string,
	responseBody interface{},
	options RequestOptions,
	diagnostics *marker,
) (*http.Response, error) {
	return transport.doRequest("GET", endpoint, nil, responseBody, options, diagnostics)
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
	url, err := transport.buildURL(endpoint, false)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, url.String(), bodyBuf)
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

func (t *transport) buildURL(path string, isRetry bool) (*url.URL, error) {
	var api string
	useDefaultAPI := isRetry && t.options.FallbackToStatsigAPI
	endpoint := strings.TrimPrefix(path, "/v1")
	if strings.Contains(endpoint, "download_config_specs") {
		if useDefaultAPI {
			api = StatsigCDN
		} else {
			api = defaultString(t.options.APIOverrides.DownloadConfigSpecs, defaultString(t.options.API, StatsigCDN))
		}
	} else if strings.Contains(endpoint, "get_id_list") {
		if useDefaultAPI {
			api = StatsigAPI
		} else {
			api = defaultString(t.options.APIOverrides.GetIDLists, defaultString(t.options.API, StatsigAPI))
		}
	} else if strings.Contains(endpoint, "log_event") {
		if useDefaultAPI {
			api = StatsigAPI
		} else {
			api = defaultString(t.options.APIOverrides.LogEvent, defaultString(t.options.API, StatsigAPI))
		}
	} else {
		if useDefaultAPI {
			api = StatsigAPI
		} else {
			api = defaultString(t.options.API, StatsigAPI)
		}
	}
	return url.Parse(strings.TrimSuffix(api, "/") + endpoint)
}

func (t *transport) updateRequestForRetry(r *http.Request) *http.Request {
	retryURL, err := t.buildURL(r.URL.Path, true)
	if err == nil && strings.Compare(r.URL.Host, retryURL.Host) != 0 {
		retryRequest, err := http.NewRequest(r.Method, retryURL.String(), r.Body)
		if err == nil {
			return retryRequest
		}
	}
	return nil
}

func (transport *transport) doRequest(
	method string,
	endpoint string,
	in interface{},
	out interface{},
	options RequestOptions,
	diagnostics *marker,
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

		if diagnostics != nil {
			diagnostics.end()

			if response != nil {
				diagnostics.success(successfulStatusCode(response.StatusCode))
				diagnostics.statusCode(response.StatusCode)
				diagnostics.sdkRegion(safeGetFirst(response.Header["X-Statsig-Region"]))
			} else {
				diagnostics.success(false)
			}
			diagnostics.mark()
		}

		if err != nil {
			return response, response != nil, err
		}

		retryRequest := transport.updateRequestForRetry(request)
		if retryRequest != nil {
			request = retryRequest
		}

		drainAndCloseBody := func() {
			if response.Body != nil {
				// Drain body to re-use the same connection
				_, _ = io.Copy(io.Discard, response.Body)
				response.Body.Close()
			}
		}
		defer drainAndCloseBody()

		if successfulStatusCode(response.StatusCode) {
			return response, false, transport.parseResponse(response, out)
		}

		return response, retryableStatusCode(response.StatusCode), fmt.Errorf("%s", response.Status)
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

func successfulStatusCode(code int) bool {
	return code >= 200 && code < 300
}
