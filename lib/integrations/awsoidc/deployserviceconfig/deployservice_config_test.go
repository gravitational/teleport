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

package deployserviceconfig

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
)

func TestDeployServiceConfig(t *testing.T) {
	t.Run("ensure log level is set to debug", func(t *testing.T) {
		base64Config, err := GenerateTeleportConfigString("host:port", "iam-token", types.Labels{})
		require.NoError(t, err)

		// Config must have the following string:
		// severity: debug

		base64SeverityDebug := base64.StdEncoding.EncodeToString([]byte("severity: debug"))
		require.Contains(t, base64Config, base64SeverityDebug)
	})
}

func TestParseResourceLabelMatchers(t *testing.T) {
	labels := types.Labels{
		"vpc":    utils.Strings{"vpc-1", "vpc-2"},
		"region": utils.Strings{"us-west-2"},
		"xyz":    utils.Strings{},
	}
	base64Config, err := GenerateTeleportConfigString("host:port", "iam-token", labels)
	require.NoError(t, err)

	t.Run("recover matching labels", func(t *testing.T) {
		gotLabels, err := ParseResourceLabelMatchers(base64Config)
		require.NoError(t, err)

		require.Equal(t, labels, gotLabels)
	})

	t.Run("fails if invalid base64 string", func(t *testing.T) {
		_, err := ParseResourceLabelMatchers("invalid base 64")
		require.ErrorContains(t, err, "base64")
	})

	t.Run("invalid yaml", func(t *testing.T) {
		input := base64.StdEncoding.EncodeToString([]byte("invalid yaml"))
		_, err := ParseResourceLabelMatchers(input)
		require.ErrorContains(t, err, "yaml")
	})

	t.Run("valid yaml but not a teleport config", func(t *testing.T) {
		yamlInput := struct {
			DBService string `yaml:"db_service"`
		}{
			DBService: "not a valid teleport config",
		}
		yamlBS, err := yaml.Marshal(yamlInput)
		require.NoError(t, err)
		input := base64.StdEncoding.EncodeToString(yamlBS)

		_, err = ParseResourceLabelMatchers(input)
		require.ErrorContains(t, err, "invalid teleport config")
	})
}
