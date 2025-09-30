/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package server

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetGCPInstallerScript(t *testing.T) {
	basicReq := func() GCPRunRequest {
		return GCPRunRequest{
			PublicProxyAddr: "proxy:443",
			ScriptName:      "scriptName",
		}
	}
	for _, tt := range []struct {
		name           string
		req            func() GCPRunRequest
		expectedScript string
	}{
		{
			name:           "basic",
			req:            basicReq,
			expectedScript: "curl -s -L https://proxy:443/v1/webapi/scripts/installer/scriptName | bash -s ",
		},
		{
			name: "with suffix installation",
			req: func() GCPRunRequest {
				req := basicReq()
				req.InstallSuffix = "suffix"
				return req
			},
			expectedScript: `export TELEPORT_INSTALL_SUFFIX="suffix"; curl -s -L https://proxy:443/v1/webapi/scripts/installer/scriptName | bash -s `,
		},
		{
			name: "with update group",
			req: func() GCPRunRequest {
				req := basicReq()
				req.UpdateGroup = "updateGroup"
				return req
			},
			expectedScript: `export TELEPORT_UPDATE_GROUP="updateGroup"; curl -s -L https://proxy:443/v1/webapi/scripts/installer/scriptName | bash -s `,
		},
		{
			name: "with install suffix and update group",
			req: func() GCPRunRequest {
				req := basicReq()
				req.InstallSuffix = "suffix"
				req.UpdateGroup = "updateGroup"
				return req
			},
			expectedScript: `export TELEPORT_INSTALL_SUFFIX="suffix" TELEPORT_UPDATE_GROUP="updateGroup"; curl -s -L https://proxy:443/v1/webapi/scripts/installer/scriptName | bash -s `,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			script := getGCPInstallerScript(tt.req())
			require.Equal(t, tt.expectedScript, script)
		})
	}
}
