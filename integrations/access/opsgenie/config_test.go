// Copyright 2023 Gravitational, Inc
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

package opsgenie

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/integrations/access/common"
)

func newMinimalValidConfig() Config {
	return Config{
		Opsgenie: common.GenericAPIConfig{
			Token: "someToken",
		},
		ClientConfig: ClientConfig{
			APIKey:      "someAPIKey",
			APIEndpoint: "someEnpoint",
			WebProxyURL: &url.URL{},
		},
	}
}

func TestInvalidConfigFailsCheck(t *testing.T) {
	t.Run("Token", func(t *testing.T) {
		cfg := newMinimalValidConfig()
		cfg.Opsgenie.Token = ""
		require.Error(t, cfg.CheckAndSetDefaults())
	})
	t.Run("API Key", func(t *testing.T) {
		cfg := newMinimalValidConfig()
		cfg.ClientConfig.APIKey = ""
		require.Error(t, cfg.CheckAndSetDefaults())
	})
	t.Run("API endpoint", func(t *testing.T) {
		cfg := newMinimalValidConfig()
		cfg.ClientConfig.APIEndpoint = ""
		require.Error(t, cfg.CheckAndSetDefaults())
	})
	t.Run("Web proxy url", func(t *testing.T) {
		cfg := newMinimalValidConfig()
		cfg.ClientConfig.WebProxyURL = nil
		require.Error(t, cfg.CheckAndSetDefaults())
	})

}

func TestConfigDefaults(t *testing.T) {
	cfg := Config{
		Opsgenie: common.GenericAPIConfig{
			Token: "someToken",
		},
		ClientConfig: ClientConfig{
			APIKey:      "someAPIKey",
			APIEndpoint: "someEnpoint",
			WebProxyURL: &url.URL{},
		},
	}
	require.NoError(t, cfg.CheckAndSetDefaults())
	require.Equal(t, "stderr", cfg.Log.Output)
	require.Equal(t, "info", cfg.Log.Severity)
}
