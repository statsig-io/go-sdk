package net

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type MockClient struct {
	Do func(req *http.Request) (*http.Response, error)
}

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
	in := &Empty{}
	var out ServerResponse
	n := New("secret-123", testServer.URL, "", "")
	err := n.RetryablePostRequest("/123", in, &out, 2)
	if err == nil {
		t.Errorf("Expected error for network request but got nil")
	}
}

func TestRetries(t *testing.T) {
	tries := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		defer func() {
			tries = tries + 1
		}()
		if tries == 0 {
			res.WriteHeader(http.StatusInternalServerError)
		} else if tries == 1 {
			output := ServerResponse{
				Name: "test",
			}
			res.WriteHeader(http.StatusOK)
			json.NewEncoder(res).Encode(output)
		}
	}))
	defer func() { testServer.Close() }()
	in := Empty{}
	var out ServerResponse
	n := New("secret-123", testServer.URL)
	err := n.RetryablePostRequest("/123", in, out, 2)
	if err != nil {
		t.Errorf("Expected successful request but got error")
	}
}
