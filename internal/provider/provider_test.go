// Copyright (c) HashiCorp, Inc.

package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type clientFunc func(req *http.Request) (*http.Response, error)

func (f clientFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
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
