// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ServerLicenseInfo is the license the gateway evaluates for the organization.
// Status says why a license is not valid, decided against the gateway clock.
type ServerLicenseInfo struct {
	Type        string `json:"type"`
	IsValid     bool   `json:"is_valid"`
	Status      string `json:"status"`
	VerifyError string `json:"verify_error"`
}

// ServerInfo is the part of GET /serverinfo this provider reads.
type ServerInfo struct {
	LicenseInfo *ServerLicenseInfo `json:"license_info"`
}

func (c *Client) GetServerInfo() (*ServerInfo, error) {
	req, err := c.newRequest("GET", "/serverinfo", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out ServerInfo
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed decoding server info, reason=%v", err)
	}
	return &out, nil
}

// LicenseCheck is the answer to "will the gateway serve the Enterprise
// routes". Unknown is true when the gateway answered /serverinfo without a
// license_info field, which a build from before that field does; the
// gateway then decides at apply. Reason is set when Enterprise is false and
// the answer is known.
type LicenseCheck struct {
	Enterprise bool
	Unknown    bool
	Reason     string
}

// CheckEnterpriseLicense asks the gateway once per client and keeps the
// answer. A plan calls it from every licensed resource, and the license does
// not change during one run.
//
// The predicate is the one the gateway's EnterpriseLicenseOnly middleware
// applies: the license verifies and its type is enterprise.
func (c *Client) CheckEnterpriseLicense() (LicenseCheck, error) {
	c.licenseOnce.Do(func() {
		info, err := c.GetServerInfo()
		if err != nil {
			c.licenseErr = err
			return
		}
		c.license = licenseCheckOf(info.LicenseInfo)
	})
	return c.license, c.licenseErr
}

func licenseCheckOf(info *ServerLicenseInfo) LicenseCheck {
	switch {
	case info == nil:
		return LicenseCheck{Unknown: true}
	case info.Type != "enterprise":
		return LicenseCheck{Reason: fmt.Sprintf("the organization's license is of type %q", info.Type)}
	case !info.IsValid:
		reason := fmt.Sprintf("the organization's Enterprise license is %s", info.Status)
		if info.VerifyError != "" {
			reason += ": " + info.VerifyError
		}
		return LicenseCheck{Reason: reason}
	}
	return LicenseCheck{Enterprise: true}
}
