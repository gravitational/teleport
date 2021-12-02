//go:build go1.16
// +build go1.16

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package runtime

import (
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/internal/shared"
)

// NewResponseError wraps the specified error with an error that provides access to an HTTP response.
// If an HTTP request returns a non-successful status code, wrap the response and the associated error
// in this error type so that callers can access the underlying *http.Response as required.
// DO NOT wrap failed HTTP requests that returned an error and no response with this type.
func NewResponseError(inner error, resp *http.Response) error {
	return shared.NewResponseError(inner, resp)
}
