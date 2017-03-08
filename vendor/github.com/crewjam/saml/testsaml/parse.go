package testsaml

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/url"
)

// ParseRedirectRequest returns the decoded SAML AuthnRequest from an HTTP-Redirect URL
func ParseRedirectRequest(u *url.URL) ([]byte, error) {
	compressedRequest, err := base64.StdEncoding.DecodeString(u.Query().Get("SAMLRequest"))
	if err != nil {
		return nil, fmt.Errorf("cannot decode request: %s", err)
	}
	buf, err := ioutil.ReadAll(flate.NewReader(bytes.NewReader(compressedRequest)))
	if err != nil {
		return nil, fmt.Errorf("cannot decompress request: %s", err)
	}
	return buf, nil
}
