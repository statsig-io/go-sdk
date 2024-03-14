package statsig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

type transport struct {
	api                       string
	apiForDownloadConfigSpecs string
	sdkKey                    string
	metadata                  statsigMetadata // Safe to read from but not thread safe to write into. If value needs to change, please ensure thread safety.
	client                    *http.Client
	options                   *Options
}

func newTransport(secret string, options *Options) *transport {
	api := defaultString(options.API, StatsigAPI)
	apiForDownloadConfigSpecs := defaultString(options.API, StatsigCDN)
	api = strings.TrimSuffix(api, "/")
	defer func() {
		if err := recover(); err != nil {
			Logger().LogError(err)
		}
	}()

	return &transport{
		api:                       api,
		apiForDownloadConfigSpecs: apiForDownloadConfigSpecs,
		metadata:                  getStatsigMetadata(),
		sdkKey:                    secret,
		client:                    &http.Client{Timeout: time.Second * 3},
		options:                   options,
	}
}

type RequestOptions struct {
	retries int
	backoff time.Duration
}

func (opts *RequestOptions) fillDefaults() {
	if opts.backoff == 0 {
		opts.backoff = time.Second
	}
}

func (transport *transport) downloadConfigSpecs(sinceTime int64, responseBody interface{}) (*http.Response, error) {
	var endpoint string
	if transport.options.DisableCDN {
		endpoint = fmt.Sprintf("/download_config_specs?sinceTime=%d", sinceTime)
	} else {
		endpoint = fmt.Sprintf("/download_config_specs/%s.json?sinceTime=%d", transport.sdkKey, sinceTime)
	}
	return transport.get(endpoint, responseBody, RequestOptions{})
}

func (transport *transport) getIdLists(responseBody interface{}) (*http.Response, error) {
	return transport.post("/get_id_lists", nil, responseBody, RequestOptions{})
}

func (transport *transport) getIdList(url string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return transport.client.Do(req)
}

func (transport *transport) post(endpoint string, body interface{}, responseBody interface{}, options RequestOptions) (*http.Response, error) {
	return transport.doRequest("POST", endpoint, body, responseBody, options)
}

func (transport *transport) get(endpoint string, responseBody interface{}, options RequestOptions) (*http.Response, error) {
	return transport.doRequest("GET", endpoint, nil, responseBody, options)
}

func (transport *transport) buildRequest(method, endpoint string, body interface{}) (*http.Request, error) {
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
	}
	req, err := http.NewRequest(method, transport.buildURL(endpoint), bodyBuf)
	if err != nil {
		return nil, err
	}

	req.Header.Add("STATSIG-API-KEY", transport.sdkKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("STATSIG-CLIENT-TIME", strconv.FormatInt(getUnixMilli(), 10))
	req.Header.Add("STATSIG-SERVER-SESSION-ID", transport.metadata.SessionID)
	req.Header.Add("STATSIG-SDK-TYPE", transport.metadata.SDKType)
	req.Header.Add("STATSIG-SDK-VERSION", transport.metadata.SDKVersion)
	return req, nil
}

func (transport *transport) buildURL(endpoint string) string {
	if strings.Contains(endpoint, "download_config_specs") {
		return transport.apiForDownloadConfigSpecs + endpoint
	} else {
		return transport.api + endpoint
	}
}

func (transport *transport) doRequest(
	method string,
	endpoint string,
	in interface{},
	out interface{},
	options RequestOptions,
) (*http.Response, error) {
	request, err := transport.buildRequest(method, endpoint, in)
	if request == nil || err != nil {
		return nil, err
	}
	options.fillDefaults()
	return retry(options.retries, options.backoff, func() (*http.Response, bool, error) {
		response, err := transport.client.Do(request)
		if err != nil {
			return response, response != nil, err
		}
		drainAndCloseBody := func() {
			if response.Body != nil {
				// Drain body to re-use the same connection
				_, _ = io.Copy(io.Discard, response.Body)
				response.Body.Close()
			}
		}
		defer drainAndCloseBody()

		if response.StatusCode >= 200 && response.StatusCode < 300 {
			return response, false, transport.parseResponse(response, out)
		}

		return response, retryableStatusCode(response.StatusCode), fmt.Errorf("http response error code: %d", response.StatusCode)
	})
}

func (transport *transport) parseResponse(response *http.Response, out interface{}) error {
	if out == nil {
		return nil
	}
	return json.NewDecoder(response.Body).Decode(&out)
}

func retry(retries int, backoff time.Duration, fn func() (*http.Response, bool, error)) (*http.Response, error) {
	for {
		if response, retry, err := fn(); retry {
			if retries <= 0 {
				return response, err
			}

			retries--
			time.Sleep(backoff)
			backoff = backoff * backoffMultiplier
		} else {
			return response, err
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
