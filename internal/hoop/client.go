// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	apiURL     string
	token      string
	httpClient HttpClient
}

func NewClient(apiURL, token string, httpClient HttpClient) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	apiURL = strings.TrimSuffix(apiURL, "/")
	return &Client{apiURL: apiURL, token: token, httpClient: httpClient}
}

func validateErr(resp *http.Response) error {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed reading response body, status=%v, reason=%v",
			resp.StatusCode, err)
	}
	return fmt.Errorf("status=%v, payload=%v", resp.StatusCode, string(data))
}
