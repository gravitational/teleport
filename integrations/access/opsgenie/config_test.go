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
