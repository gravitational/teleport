//go:build go1.16
// +build go1.16

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azcore

import (
	"context"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
)

// Credential represents any credential type.
type Credential interface {
	// AuthenticationPolicy returns a policy that requests the credential and applies it to the HTTP request.
	NewAuthenticationPolicy(options runtime.AuthenticationOptions) policy.Policy
}

// credentialFunc is a type that implements the Credential interface.
// Use this type when implementing a stateless credential as a first-class function.
type credentialFunc func(options runtime.AuthenticationOptions) policy.Policy

// AuthenticationPolicy implements the Credential interface on credentialFunc.
func (cf credentialFunc) NewAuthenticationPolicy(options runtime.AuthenticationOptions) policy.Policy {
	return cf(options)
}

// TokenCredential represents a credential capable of providing an OAuth token.
type TokenCredential interface {
	Credential
	// GetToken requests an access token for the specified set of scopes.
	GetToken(ctx context.Context, options policy.TokenRequestOptions) (*AccessToken, error)
}

// AccessToken represents an Azure service bearer access token with expiry information.
type AccessToken struct {
	Token     string
	ExpiresOn time.Time
}
