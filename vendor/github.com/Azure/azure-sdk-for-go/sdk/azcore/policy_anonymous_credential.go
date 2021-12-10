//go:build go1.16
// +build go1.16

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azcore

import (
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/internal/pipeline"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
)

func anonCredAuthPolicyFunc(runtime.AuthenticationOptions) policy.Policy {
	return pipeline.PolicyFunc(anonCredPolicyFunc)
}

func anonCredPolicyFunc(req *policy.Request) (*http.Response, error) {
	return req.Next()
}

// NewAnonymousCredential is for use with HTTP(S) requests that read public resource
// or for use with Shared Access Signatures (SAS).
func NewAnonymousCredential() Credential {
	return credentialFunc(anonCredAuthPolicyFunc)
}
