/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package azure

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// responseWithBody builds an *http.Response carrying the given JSON body so
// ConvertResponseError's BadRequest branch can exercise errorDetails decoding.
func responseWithBody(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestConvertResponseError(t *testing.T) {
	t.Parallel()

	badRequestErrorWithUnknownCode := &azcore.ResponseError{
		StatusCode: http.StatusBadRequest,
		RawResponse: responseWithBody(http.StatusBadRequest,
			`{"error":{"details":[{"code":"SomeOtherCode","message":"x"}]}}`),
	}
	// The bodyclose linter trips if we don't explicitly close the body, even though it's a no-op in this case.
	badRequestErrorWithUnknownCode.RawResponse.Body.Close()

	badRequestErrorWithNilBody := &azcore.ResponseError{StatusCode: http.StatusBadRequest}

	badRequestErrorWithInvalidJSON := &azcore.ResponseError{
		StatusCode:  http.StatusBadRequest,
		RawResponse: responseWithBody(http.StatusBadRequest, `not-json`),
	}

	badRequestErrorWithEmptyDetails := &azcore.ResponseError{
		StatusCode:  http.StatusBadRequest,
		RawResponse: responseWithBody(http.StatusBadRequest, `{"error":{"details":[]}}`),
	}

	internalServerError := &azcore.ResponseError{StatusCode: http.StatusInternalServerError}

	nonAzureError := errors.New("non-azure error")

	cases := []struct {
		name  string
		err   error
		check require.ErrorAssertionFunc
	}{
		{
			name:  "nil input returns nil",
			check: require.NoError,
		},
		{
			name: "403 maps to AccessDenied",
			err:  &azcore.ResponseError{StatusCode: http.StatusForbidden},
			check: func(tt require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "got %T", err)
			},
		},
		{
			name: "409 maps to AlreadyExists",
			err:  &azcore.ResponseError{StatusCode: http.StatusConflict},
			check: func(tt require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAlreadyExists(err), "got %T", err)
			},
		},
		{
			name: "404 maps to NotFound",
			err:  &azcore.ResponseError{StatusCode: http.StatusNotFound},
			check: func(tt require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err), "got %T", err)
			},
		},
		{
			name: "429 maps to LimitExceeded",
			err:  &azcore.ResponseError{StatusCode: http.StatusTooManyRequests},
			check: func(tt require.TestingT, err error, i ...any) {
				require.True(t, trace.IsLimitExceeded(err), "got %T", err)
			},
		},
		{
			name: "400 SubscriptionsContainInvalidGuids maps to BadParameter",
			err: &azcore.ResponseError{
				StatusCode: http.StatusBadRequest,
				RawResponse: responseWithBody(http.StatusBadRequest,
					`{"error":{"details":[{"code":"SubscriptionsContainInvalidGuids","message":"bad guid"}]}}`),
			},
			check: func(tt require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err), "got %T", err)
			},
		},
		{
			name: "400 NoValidSubscriptionsInQueryRequest maps to AccessDenied",
			err: &azcore.ResponseError{
				StatusCode: http.StatusBadRequest,
				RawResponse: responseWithBody(http.StatusBadRequest,
					`{"error":{"details":[{"code":"NoValidSubscriptionsInQueryRequest","message":"no subs"}]}}`),
			},
			check: func(tt require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "got %T", err)
			},
		},
		{
			name: "400 with unknown details code returns original error",
			err:  badRequestErrorWithUnknownCode,
			check: func(tt require.TestingT, err error, i ...any) {
				require.Same(t, badRequestErrorWithUnknownCode, err, "unknown 400 code must pass through unmodified")
			},
		},
		{
			name: "400 with nil RawResponse returns original error",
			err:  badRequestErrorWithNilBody,
			check: func(tt require.TestingT, err error, i ...any) {
				require.Same(t, badRequestErrorWithNilBody, err, "400 without RawResponse must pass through unmodified")
			},
		},
		{
			name: "400 with invalid JSON returns original error",
			err:  badRequestErrorWithInvalidJSON,
			check: func(tt require.TestingT, err error, i ...any) {
				require.Same(t, badRequestErrorWithInvalidJSON, err, "400 with undecodable body must pass through unmodified")
			},
		},
		{
			name: "400 with empty details returns original error",
			err:  badRequestErrorWithEmptyDetails,
			check: func(tt require.TestingT, err error, i ...any) {
				require.Same(t, badRequestErrorWithEmptyDetails, err, "400 with no details must pass through unmodified")
			},
		},
		{
			name: "unhandled status code returns original error",
			err:  internalServerError,
			check: func(tt require.TestingT, err error, i ...any) {
				require.Same(t, internalServerError, err, "unhandled status code must pass through unmodified")
			},
		},
		{
			name: "AuthenticationFailedError maps to AccessDenied",
			err:  &azidentity.AuthenticationFailedError{},
			check: func(tt require.TestingT, err error, i ...any) {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err), "got %T: %v", err, err)
			},
		},
		{
			name: "non-Azure error returns original error",
			err:  nonAzureError,
			check: func(tt require.TestingT, err error, i ...any) {
				require.Same(t, nonAzureError, err, "unrelated error types must pass through unmodified")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ConvertResponseError(tc.err)
			tc.check(t, got)
		})
	}
}

func TestConvertResponseError_WrappedErrors(t *testing.T) {
	t.Parallel()

	t.Run("wrapped ResponseError still maps by status", func(t *testing.T) {
		wrapped := trace.Wrap(&azcore.ResponseError{StatusCode: http.StatusForbidden}, "while listing")
		got := ConvertResponseError(wrapped)
		require.Error(t, got)
		require.True(t, trace.IsAccessDenied(got), "got %T: %v", got, got)
	})

	t.Run("wrapped AuthenticationFailedError still maps to AccessDenied", func(t *testing.T) {
		wrapped := trace.Wrap(&azidentity.AuthenticationFailedError{}, "while authenticating")
		got := ConvertResponseError(wrapped)
		require.Error(t, got)
		require.True(t, trace.IsAccessDenied(got), "got %T: %v", got, got)
	})
}
