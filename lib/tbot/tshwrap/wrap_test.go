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

package tshwrap

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

// TestGetEnvForTSH ensures we generate a valid minimum subset of environment
// parameters needed for tsh wrappers to work.
func TestGetEnvForTSH(t *testing.T) {
	p := "/foo"

	expected := map[string]string{
		client.VirtualPathEnvName(client.VirtualPathKey, nil):      filepath.Join(p, identity.PrivateKeyKey),
		client.VirtualPathEnvName(client.VirtualPathDatabase, nil): filepath.Join(p, identity.TLSCertKey),
		client.VirtualPathEnvName(client.VirtualPathAppCert, nil):  filepath.Join(p, identity.TLSCertKey),

		client.VirtualPathEnvName(client.VirtualPathCA, client.VirtualPathCAParams(types.UserCA)):     filepath.Join(p, config.UserCAPath),
		client.VirtualPathEnvName(client.VirtualPathCA, client.VirtualPathCAParams(types.HostCA)):     filepath.Join(p, config.HostCAPath),
		client.VirtualPathEnvName(client.VirtualPathCA, client.VirtualPathCAParams(types.DatabaseCA)): filepath.Join(p, config.DatabaseCAPath),
	}

	env, err := GetEnvForTSH(p)
	require.NoError(t, err)
	for k, v := range expected {
		assert.Equal(t, v, env[k])
	}
}

func TestGetDestinationDirectory(t *testing.T) {
	config := func(outputCount int) *config.BotConfig {
		cfg := &config.BotConfig{}
		for i := 0; i < outputCount; i++ {
			cfg.Services = append(cfg.Services, &config.IdentityOutput{
				Destination: &config.DestinationDirectory{
					Path: fmt.Sprintf("/from-bot-config%d", i),
				},
			})
		}
		require.NoError(t, cfg.CheckAndSetDefaults())
		return cfg
	}
	t.Run("one output configured", func(t *testing.T) {
		dest, err := GetDestinationDirectory("", config(1))
		require.NoError(t, err)
		require.Equal(t, "/from-bot-config0", dest.Path)
	})
	t.Run("no outputs specified", func(t *testing.T) {
		_, err := GetDestinationDirectory("", config(0))
		require.ErrorContains(t, err, "either --destination-dir or a config file containing an output or service must be specified")
	})
	t.Run("multiple outputs specified", func(t *testing.T) {
		_, err := GetDestinationDirectory("", config(2))
		require.ErrorContains(t, err, "the config file contains multiple outputs and services; a --destination-dir must be specified")
	})
}
