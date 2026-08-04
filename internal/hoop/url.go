// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"errors"
	"net/url"
)

// redactedURLPlaceholder stands in for a URL that could not be parsed, and
// therefore could not be stripped of its credentials.
const redactedURLPlaceholder = "(unparsable url)"

// redactURL removes every credential-bearing component from rawURL so the
// result is safe to put in a log record or an error message.
//
// api_url is not a sensitive value in the provider schema and is normally a
// plain gateway endpoint, but nothing stops an operator from pointing it at a
// URL carrying userinfo or a secret query parameter. Neither survives here:
// what comes back is scheme, host and path.
func redactURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return redactedURLPlaceholder
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// urlErrorReason unwraps the *url.Error that net/url reports on a malformed URL
// and yields only its reason.
//
// The wrapper's own message embeds the whole URL — net/url has no equivalent of
// net/http's stripPassword, so it prints userinfo in the clear. A single
// control character in a connection name is enough to reach this path, and the
// resulting error is surfaced to the operator as a Terraform diagnostic.
func urlErrorReason(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}
