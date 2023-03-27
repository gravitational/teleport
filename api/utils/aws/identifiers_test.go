/*
Copyright 2022 Gravitational, Inc.

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
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestIsValidAccountID(t *testing.T) {
	isBadParamErrFn := func(tt require.TestingT, err error, i ...interface{}) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	for _, tt := range []struct {
		name      string
		accountID string
		errCheck  require.ErrorAssertionFunc
	}{
		{
			name:      "valid account id",
			accountID: "123456789012",
			errCheck:  require.NoError,
		},
		{
			name:      "empty",
			accountID: "",
			errCheck:  isBadParamErrFn,
		},
		{
			name:      "less digits",
			accountID: "12345678901",
			errCheck:  isBadParamErrFn,
		},
		{
			name:      "more digits",
			accountID: "1234567890123",
			errCheck:  isBadParamErrFn,
		},
		{
			name:      "invalid chars",
			accountID: "12345678901A",
			errCheck:  isBadParamErrFn,
		},
		{
			name:      "invalid chars with emojis",
			accountID: "12345678901✅",
			errCheck:  isBadParamErrFn,
		},
		{
			name:      "unicode digit is invalid",
			accountID: "123456789৩", // ৩ is a valid unicode digit and its len("৩") is 3
			errCheck:  isBadParamErrFn,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.errCheck(t, IsValidAccountID(tt.accountID))
		})
	}
}
