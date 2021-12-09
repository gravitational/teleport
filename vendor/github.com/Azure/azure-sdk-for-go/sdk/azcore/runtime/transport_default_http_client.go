//go:build go1.16
// +build go1.16

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package runtime

import (
	"crypto/tls"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

var defaultHTTPClient *http.Client

func init() {
	defaultTransport := http.DefaultTransport.(*http.Transport).Clone()
	defaultTransport.TLSClientConfig.MinVersion = tls.VersionTLS12
	defaultHTTPClient = &http.Client{
		Transport: defaultTransport,
	}
}

// AuthenticationOptions contains various options used to create a credential policy.
type AuthenticationOptions struct {
	// TokenRequest is a TokenRequestOptions that includes a scopes field which contains
	// the list of OAuth2 authentication scopes used when requesting a token.
	// This field is ignored for other forms of authentication (e.g. shared key).
	TokenRequest policy.TokenRequestOptions
	// AuxiliaryTenants contains a list of additional tenant IDs to be used to authenticate
	// in cross-tenant applications.
	AuxiliaryTenants []string
}
