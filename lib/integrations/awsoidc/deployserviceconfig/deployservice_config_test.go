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

	"github.com/gravitational/teleport/api/types"
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
