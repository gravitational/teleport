//go:build go1.16
// +build go1.16

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azcore

import (
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/internal/shared"
	"github.com/Azure/azure-sdk-for-go/sdk/internal/errorinfo"
)

// HTTPResponse provides access to an HTTP response when available.
// Errors returned from failed API calls will implement this interface.
// Use errors.As() to access this interface in the error chain.
// If there was no HTTP response then this interface will be omitted
// from any error in the chain.
type HTTPResponse interface {
	RawResponse() *http.Response
}

var _ HTTPResponse = (*shared.ResponseError)(nil)
var _ errorinfo.NonRetriable = (*shared.ResponseError)(nil)
