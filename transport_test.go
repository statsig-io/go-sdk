package statsig

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type Empty struct{}

type ServerResponse struct {
	Name string `json:"name"`
}

func TestNonRetryable(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if req.Method != "POST" {
			t.Errorf("Expected ‘POST’ request, got '%s'", req.Method)
		}

		res.WriteHeader(http.StatusNotFound)
	}))
	defer testServer.Close()
	in := Empty{}
	var out ServerResponse
	opt := &Options{
		API: testServer.URL,
	}
	n := newTransport("secret-123", opt)
	_, err := n.post("/123", in, &out, RequestOptions{retries: 2}, nil)
	if err == nil {
		t.Errorf("Expected error for network request but got nil")
	}
}

func TestLocalMode(t *testing.T) {
	hit := false
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		hit = true
		res.WriteHeader(http.StatusNotFound)
	}))
	defer testServer.Close()
	in := Empty{}
	var out ServerResponse
	opt := &Options{
		API:       testServer.URL,
		LocalMode: true,
	}
	n := newTransport("secret-123", opt)
	_, err := n.post("/123", in, &out, RequestOptions{retries: 2}, nil)
	if err != nil {
		t.Errorf("Expected no error for network request")
	}
	if hit {
		t.Errorf("Expected transport class not to hit the server")
	}
}

func TestRetries(t *testing.T) {
	tries := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		defer func() {
			tries = tries + 1
		}()
		switch tries {
		case 0:
			res.WriteHeader(http.StatusInternalServerError)
		case 1:
			output := ServerResponse{
				Name: "test",
			}
			res.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(res).Encode(output)
		}
	}))
	defer func() { testServer.Close() }()
	in := Empty{}
	var out ServerResponse
	opt := &Options{
		API: testServer.URL,
	}
	n := newTransport("secret-123", opt)
	_, err := n.post("/123", in, out, RequestOptions{retries: 2}, nil)
	if err != nil {
		t.Errorf("Expected successful request but got error")
	}
}

func TestProxy(t *testing.T) {
	testServerHit := false
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		testServerHit = true
	}))
	defer testServer.Close()
	in := Empty{}
	var out ServerResponse
	url, _ := url.Parse(testServer.URL)
	opt := &Options{
		Transport: &http.Transport{Proxy: http.ProxyURL(url)},
	}
	n := newTransport("secret-123", opt)
	_, _ = n.post("/123", in, &out, RequestOptions{}, nil)
	if !testServerHit {
		t.Errorf("Expected request to hit proxy server")
	}
}
