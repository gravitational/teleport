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
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestInstallerScript(t *testing.T) {
	basicParams := func() *types.InstallerParams {
		return &types.InstallerParams{
			PublicProxyAddr: "proxy.example.com:443",
			ScriptName:      "scriptName",
			JoinToken:       "my-token",
		}
	}
	for _, tt := range []struct {
		name                   string
		req                    func() *types.InstallerParams
		withOptions            []scriptOption
		errCheck               require.ErrorAssertionFunc
		expectedScript         string
		expectedScriptContains string
	}{
		{
			name:           "basic",
			req:            basicParams,
			errCheck:       require.NoError,
			expectedScript: "curl -s -L https://proxy.example.com:443/v1/webapi/scripts/installer/scriptName | bash -s my-token",
		},
		{
			name:                   "with nonce to ensure script is unique",
			req:                    basicParams,
			withOptions:            []scriptOption{withNonceComment()},
			errCheck:               require.NoError,
			expectedScriptContains: "curl -s -L https://proxy.example.com:443/v1/webapi/scripts/installer/scriptName | bash -s my-token # ",
		},
		{
			name: "with suffix installation",
			req: func() *types.InstallerParams {
				req := basicParams()
				req.Suffix = "suffix"
				return req
			},
			errCheck:       require.NoError,
			expectedScript: "export TELEPORT_INSTALL_SUFFIX=suffix; curl -s -L https://proxy.example.com:443/v1/webapi/scripts/installer/scriptName | bash -s my-token",
		},
		{
			name: "with update group",
			req: func() *types.InstallerParams {
				req := basicParams()
				req.UpdateGroup = "updateGroup"
				return req
			},
			errCheck:       require.NoError,
			expectedScript: "export TELEPORT_UPDATE_GROUP=updateGroup; curl -s -L https://proxy.example.com:443/v1/webapi/scripts/installer/scriptName | bash -s my-token",
		},
		{
			name: "with install suffix and update group",
			req: func() *types.InstallerParams {
				req := basicParams()
				req.Suffix = "suffix"
				req.UpdateGroup = "updateGroup"
				return req
			},
			errCheck:       require.NoError,
			expectedScript: "export TELEPORT_INSTALL_SUFFIX=suffix TELEPORT_UPDATE_GROUP=updateGroup; curl -s -L https://proxy.example.com:443/v1/webapi/scripts/installer/scriptName | bash -s my-token",
		},
		{
			name: "missing public proxy address but getter was set up",
			req: func() *types.InstallerParams {
				req := basicParams()
				req.PublicProxyAddr = ""
				return req
			},
			withOptions: []scriptOption{
				withProxyAddrGetter(func(ctx context.Context) (string, error) {
					return "proxy2.example.com", nil
				}),
			},
			errCheck:       require.NoError,
			expectedScript: "curl -s -L https://proxy2.example.com/v1/webapi/scripts/installer/scriptName | bash -s my-token",
		},
		{
			name: "proxy addr is missing but getter returns an error",
			req: func() *types.InstallerParams {
				req := basicParams()
				req.PublicProxyAddr = ""
				return req
			},
			withOptions: []scriptOption{
				withProxyAddrGetter(func(ctx context.Context) (string, error) {
					return "", trace.NotFound("proxy service is not yet running")
				}),
			},
			errCheck: require.Error,
		},
		{
			name: "proxy addr is missing and getter is missing",
			req: func() *types.InstallerParams {
				req := basicParams()
				req.PublicProxyAddr = ""
				return req
			},
			errCheck: require.Error,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			script, err := installerScript(t.Context(), tt.req(), tt.withOptions...)
			tt.errCheck(t, err)

			if tt.expectedScriptContains != "" {
				require.Contains(t, script, tt.expectedScriptContains)
			}

			if tt.expectedScript != "" {
				require.Equal(t, tt.expectedScript, script)
			}
		})
	}
}
