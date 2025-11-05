// Copyright 2025 Gravitational, Inc
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

func TestHTTPProxySettingsCheckAndSetDefaults(t *testing.T) {
	for _, tt := range []struct {
		name     string
		in       *HTTPProxySettings
		errCheck require.ErrorAssertionFunc
	}{
		{
			name: "valid",
			in: &HTTPProxySettings{
				HTTPProxy:  "http://proxy.example.com:8080",
				HTTPSProxy: "http://proxy.example.com:8080",
				NoProxy:    "internal.local",
			},
			errCheck: require.NoError,
		},
		{
			name: "invalid http_proxy",
			in: &HTTPProxySettings{
				HTTPProxy: "not a valid url",
			},
			errCheck: require.Error,
		},
		{
			name: "invalid https_proxy",
			in: &HTTPProxySettings{
				HTTPSProxy: "not a valid url",
			},
			errCheck: require.Error,
		},
		{
			name: "no_proxy is always valid",
			in: &HTTPProxySettings{
				NoProxy: "internal.local, ::1/128,",
			},
			errCheck: require.NoError,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.CheckAndSetDefaults()
			tt.errCheck(t, err)
		})
	}
}
