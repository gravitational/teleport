// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateMSGraphEndpoints(t *testing.T) {
	for _, tt := range []struct {
		name          string
		loginEndpoint string
		graphEndpoint string
		errAssertion  require.ErrorAssertionFunc
	}{
		{
			name:          "valid endpoints",
			loginEndpoint: "https://login.microsoftonline.com",
			graphEndpoint: "https://graph.microsoft.com",
			errAssertion:  require.NoError,
		},
		{
			name:          "empty value is permitted",
			loginEndpoint: "",
			graphEndpoint: "",
			errAssertion:  require.NoError,
		},
		{
			name:          "login and graph endpoint pair is not matched",
			loginEndpoint: "https://login.microsoftonline.com",
			graphEndpoint: "",
			errAssertion:  require.NoError,
		},
		{
			name:          "empty login endpoint and invalid graph endpoint not allowed",
			loginEndpoint: "",
			graphEndpoint: "https://graph.windows.net",
			errAssertion:  require.Error,
		},
		{
			name:          "invalid login and graph endpoint",
			loginEndpoint: "https://login.microsoft.com",
			graphEndpoint: "https://graph.windows.net",
			errAssertion:  require.Error,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.errAssertion(t, ValidateMSGraphAndLoginEndpoints(tt.loginEndpoint, tt.graphEndpoint))
		})
	}
}

func TestValidateMSGraphEndpoint(t *testing.T) {
	for _, tt := range []struct {
		name          string
		graphEndpoint string
		errAssertion  require.ErrorAssertionFunc
	}{
		{
			name:          "valid endpoint",
			graphEndpoint: "https://graph.microsoft.com",
			errAssertion:  require.NoError,
		},
		{
			name:          "invalid endpoint",
			graphEndpoint: "https://graph.windows.net",
			errAssertion:  require.Error,
		},
		{
			name:          "empty value",
			graphEndpoint: "",
			errAssertion:  require.NoError,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.errAssertion(t, ValidateMSGraphEndpoint(tt.graphEndpoint))
		})
	}
}

func TestValidateMSLoginEndpoint(t *testing.T) {
	for _, tt := range []struct {
		name          string
		loginEndpoint string
		errAssertion  require.ErrorAssertionFunc
	}{
		{
			name:          "valid endpoint",
			loginEndpoint: "https://login.microsoftonline.com",
			errAssertion:  require.NoError,
		},
		{
			name:          "invalid endpoint",
			loginEndpoint: "https://login.microsoft.com",
			errAssertion:  require.Error,
		},
		{
			name:          "empty value",
			loginEndpoint: "",
			errAssertion:  require.NoError,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.errAssertion(t, ValidateMSLoginEndpoint(tt.loginEndpoint))
		})
	}
}
