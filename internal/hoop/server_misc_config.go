// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ServerMiscConfig is the gateway's one proxy servers row. PUT replaces the
// whole row: a block left out is stored empty. A nil block is "none", a block
// with an empty listen_address stops that proxy.
//
// The provider manages the HTTP proxy block only. The other three are raw
// JSON so a read-modify-write passes them back byte for byte: the SSH block
// carries a host key and a user mapping, and a typed struct would drop any
// field a newer gateway adds.
type ServerMiscConfig struct {
	GrpcServerURL         string             `json:"grpc_server_url"`
	PostgresServerConfig  json.RawMessage    `json:"postgres_server_config"`
	SSHServerConfig       json.RawMessage    `json:"ssh_server_config"`
	RDPServerConfig       json.RawMessage    `json:"rdp_server_config"`
	HttpProxyServerConfig *ServerProxyConfig `json:"http_proxy_server_config"`
}

type ServerProxyConfig struct {
	ListenAddress string `json:"listen_address"`
}

// GetServerMiscConfig reads the row. With no row stored the gateway answers
// the gRPC URL it runs from its environment and no blocks.
func (c *Client) GetServerMiscConfig() (*ServerMiscConfig, error) {
	req, err := c.newRequest("GET", "/serverconfig/misc", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out ServerMiscConfig
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed decoding server misc config, reason=%v", err)
	}
	return &out, nil
}

// UpdateServerMiscConfig replaces the row and returns what the gateway stored.
func (c *Client) UpdateServerMiscConfig(cfg *ServerMiscConfig) (*ServerMiscConfig, error) {
	body, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal server misc config, reason=%v", err)
	}
	req, err := c.newRequest("PUT", "/serverconfig/misc", body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out ServerMiscConfig
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed decoding server misc config, reason=%v", err)
	}
	return &out, nil
}
