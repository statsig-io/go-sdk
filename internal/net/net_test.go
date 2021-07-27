package net

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type MockClient struct {
	Do func(req *http.Request) (*http.Response, error)
}

type Empty = struct{}

func Test(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	fmt.Println(testServer.URL)
	in := Empty{}
	out := Empty{}
	n := New("secret-123", testServer.URL)
	err := n.PostRequest("123", in, out)
	if err == nil {
		t.Errorf("Expected error for network request but got nil")
	}
}
