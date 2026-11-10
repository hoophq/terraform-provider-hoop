// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ServerMiscConfig is the gateway's one proxy servers row. PUT replaces the
// whole row: a block left out is stored empty. A nil block is "none", a block
// with an empty listen_address stops that proxy. The gRPC URL is not managed
// by the provider and is passed back as read.
type ServerMiscConfig struct {
	GrpcServerURL         string             `json:"grpc_server_url"`
	PostgresServerConfig  *ServerProxyConfig `json:"postgres_server_config"`
	SSHServerConfig       *ServerSSHConfig   `json:"ssh_server_config"`
	RDPServerConfig       *ServerProxyConfig `json:"rdp_server_config"`
	HttpProxyServerConfig *ServerProxyConfig `json:"http_proxy_server_config"`
}

type ServerProxyConfig struct {
	ListenAddress string `json:"listen_address"`
}

// ServerSSHConfig is the SSH proxy block. An empty HostsKey makes the gateway
// generate one when the proxy starts, and answer it.
type ServerSSHConfig struct {
	ListenAddress string            `json:"listen_address"`
	HostsKey      string            `json:"hosts_key,omitempty"`
	TrustedCAs    []string          `json:"trusted_cas,omitempty"`
	UserMapping   *ServerSSHUserMap `json:"user_mapping,omitempty"`
}

// ServerSSHUserMap names the certificate field and the user field the SSH
// proxy matches to sign a certificate holder in.
type ServerSSHUserMap struct {
	CertAttribute string `json:"cert_attr"`
	UserAttribute string `json:"user_attr"`
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
