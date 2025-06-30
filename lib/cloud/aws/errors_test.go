/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package aws

import (
	"errors"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestConvertRequestFailureError(t *testing.T) {
	t.Parallel()

	fakeRequestID := "11111111-2222-3333-3333-333333333334"

	newResponseError := func(code int) error {
		return &awshttp.ResponseError{
			RequestID: fakeRequestID,
			ResponseError: &smithyhttp.ResponseError{
				Response: &smithyhttp.Response{Response: &http.Response{
					StatusCode: code,
				}},
				Err: trace.Errorf("inner"),
			},
		}
	}

	tests := []struct {
		name           string
		inputError     error
		wantUnmodified bool
		wantIsError    func(error) bool
	}{
		{
			name:        "StatusForbidden",
			inputError:  newResponseError(http.StatusForbidden),
			wantIsError: trace.IsAccessDenied,
		},
		{
			name:        "StatusConflict",
			inputError:  newResponseError(http.StatusConflict),
			wantIsError: trace.IsAlreadyExists,
		},
		{
			name:        "StatusNotFound",
			inputError:  newResponseError(http.StatusNotFound),
			wantIsError: trace.IsNotFound,
		},
		{
			name:           "StatusBadRequest",
			inputError:     newResponseError(http.StatusBadRequest),
			wantUnmodified: true,
		},
		{
			name: "StatusBadRequest with AccessDeniedException",
			inputError: &awshttp.ResponseError{
				RequestID: fakeRequestID,
				ResponseError: &smithyhttp.ResponseError{
					Response: &smithyhttp.Response{Response: &http.Response{
						StatusCode: http.StatusBadRequest,
					}},
					Err: trace.Errorf("AccessDeniedException"),
				},
			},
			wantIsError: trace.IsAccessDenied,
		},
		{
			name:           "not AWS error",
			inputError:     errors.New("not-aws-error"),
			wantUnmodified: true,
		},
		{
			name: "v2 sdk error for ecs ClusterNotFoundException",
			inputError: &awshttp.ResponseError{
				RequestID: fakeRequestID,
				ResponseError: &smithyhttp.ResponseError{
					Response: &smithyhttp.Response{Response: &http.Response{
						StatusCode: http.StatusBadRequest,
					}},
					Err: trace.Errorf("ClusterNotFoundException"),
				},
			},
			wantIsError: trace.IsNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ConvertRequestFailureError(test.inputError)

			if test.wantUnmodified {
				require.Equal(t, test.inputError, err)
			} else {
				require.True(t, test.wantIsError(err))
			}
		})
	}
}

func TestConvertIAMv2Error(t *testing.T) {
	for _, tt := range []struct {
		name     string
		inErr    error
		errCheck require.ErrorAssertionFunc
	}{
		{
			name:     "no error",
			inErr:    nil,
			errCheck: require.NoError,
		},
		{
			name: "resource already exists",
			inErr: &iamtypes.EntityAlreadyExistsException{
				Message: aws.String("resource exists"),
			},
			errCheck: func(tt require.TestingT, err error, i ...any) {
				require.True(tt, trace.IsAlreadyExists(err), "expected trace.AlreadyExists error, got %v", err)
			},
		},
		{
			name: "resource already exists",
			inErr: &iamtypes.NoSuchEntityException{
				Message: aws.String("resource not found"),
			},
			errCheck: func(tt require.TestingT, err error, i ...any) {
				require.True(tt, trace.IsNotFound(err), "expected trace.NotFound error, got %v", err)
			},
		},
		{
			name: "invalid policy document",
			inErr: &iamtypes.MalformedPolicyDocumentException{
				Message: aws.String("malformed document"),
			},
			errCheck: func(tt require.TestingT, err error, i ...any) {
				require.True(tt, trace.IsBadParameter(err), "expected trace.BadParameter error, got %v", err)
			},
		},
		{
			name: "unauthorized",
			inErr: &awshttp.ResponseError{
				ResponseError: &smithyhttp.ResponseError{
					Response: &smithyhttp.Response{Response: &http.Response{
						StatusCode: http.StatusForbidden,
					}},
					Err: trace.Errorf(""),
				},
			},
			errCheck: func(tt require.TestingT, err error, i ...any) {
				require.True(tt, trace.IsAccessDenied(err), "expected trace.AccessDenied error, got %v", err)
			},
		},
		{
			name: "not found",
			inErr: &awshttp.ResponseError{
				ResponseError: &smithyhttp.ResponseError{
					Response: &smithyhttp.Response{Response: &http.Response{
						StatusCode: http.StatusNotFound,
					}},
					Err: trace.Errorf(""),
				},
			},
			errCheck: func(tt require.TestingT, err error, i ...any) {
				require.True(tt, trace.IsNotFound(err), "expected trace.NotFound error, got %v", err)
			},
		},
		{
			name: "resource already exists",
			inErr: &awshttp.ResponseError{
				ResponseError: &smithyhttp.ResponseError{
					Response: &smithyhttp.Response{Response: &http.Response{
						StatusCode: http.StatusConflict,
					}},
					Err: trace.Errorf(""),
				},
			},
			errCheck: func(tt require.TestingT, err error, i ...any) {
				require.True(tt, trace.IsAlreadyExists(err), "expected trace.AlreadyExists error, got %v", err)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.errCheck(t, ConvertIAMError(tt.inErr))
		})
	}
}
