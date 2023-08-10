/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aws

import (
	"errors"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestConvertRequestFailureError(t *testing.T) {
	t.Parallel()

	fakeRequestID := "11111111-2222-3333-3333-333333333334"

	tests := []struct {
		name           string
		inputError     error
		wantUnmodified bool
		wantIsError    func(error) bool
	}{
		{
			name:        "StatusForbidden",
			inputError:  awserr.NewRequestFailure(awserr.New("code", "message", nil), http.StatusForbidden, fakeRequestID),
			wantIsError: trace.IsAccessDenied,
		},
		{
			name:        "StatusConflict",
			inputError:  awserr.NewRequestFailure(awserr.New("code", "message", nil), http.StatusConflict, fakeRequestID),
			wantIsError: trace.IsAlreadyExists,
		},
		{
			name:        "StatusNotFound",
			inputError:  awserr.NewRequestFailure(awserr.New("code", "message", nil), http.StatusNotFound, fakeRequestID),
			wantIsError: trace.IsNotFound,
		},
		{
			name:           "StatusBadRequest",
			inputError:     awserr.NewRequestFailure(awserr.New("code", "message", nil), http.StatusBadRequest, fakeRequestID),
			wantUnmodified: true,
		},
		{
			name:        "StatusBadRequest with AccessDeniedException",
			inputError:  awserr.NewRequestFailure(awserr.New("AccessDeniedException", "message", nil), http.StatusBadRequest, fakeRequestID),
			wantIsError: trace.IsAccessDenied,
		},
		{
			name:           "not AWS error",
			inputError:     errors.New("not-aws-error"),
			wantUnmodified: true,
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
