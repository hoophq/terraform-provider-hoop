// Copyright (c) HashiCorp, Inc.

package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
)

type clientFunc func(req *http.Request) (*http.Response, error)

func (f clientFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

// requireAuthHeaders wraps a fake gateway and asserts every request carries the
// expected credential header, and only that one. The gateway auth middleware
// short-circuits on the Api-Key header, so the two schemes must never be sent
// together.
func requireAuthHeaders(t *testing.T, wantHeader, wantValue string, next clientFunc) clientFunc {
	t.Helper()
	otherHeader := "Authorization"
	if wantHeader == otherHeader {
		otherHeader = "Api-Key"
	}
	return clientFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get(wantHeader); got != wantValue {
			t.Errorf("%s %s: %s header: got %q, want %q", req.Method, req.URL.Path, wantHeader, got, wantValue)
		}
		if got := req.Header.Get(otherHeader); got != "" {
			t.Errorf("%s %s: %s header must not be sent, got %q", req.Method, req.URL.Path, otherHeader, got)
		}
		return next(req)
	})
}

func httpTestErr(statusCode int, contentBody string, v ...any) *http.Response {
	content := fmt.Sprintf(`{"message": %q}`, fmt.Sprintf(contentBody, v...))
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(content)),
	}
}

func httpTestOk(statusCode int, obj any) *http.Response {
	buf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(buf).Encode(obj); err != nil {
		panic(err)
	}
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(buf),
	}
}
