// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	authe "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestSubCACommands(t *testing.T) {
	t.Parallel()

	authClient := setupSubCASuite(t)

	const wantEmptyList = "[]\n"
	t.Run("get", func(t *testing.T) {
		stdout := runSubCAResourceCommand(t, authClient, "get", "ca_overrides", "--format=json")
		assert.Equal(t, wantEmptyList, stdout)
	})

	cn, err := authClient.GetClusterName(t.Context())
	require.NoError(t, err)
	clusterName := cn.GetClusterName()

	// Prepare an empty CA override. That's enough to assert that the commands work.
	const caType = "windows"
	var caOverridePath string
	{
		tempDir := t.TempDir()
		caOverridePath = filepath.Join(tempDir, "ca_override.yaml")
		data := `
kind: cert_authority_override
sub_kind: ` + caType + `
version: v1
metadata:
  name: ` + clusterName + `
spec: {}
`
		require.NoError(t, os.WriteFile(caOverridePath, []byte(data), 0644))
	}

	// "tctl get" for CA overrides is based on a cached read, so loop with
	// eventually in case the cache is not up-to-date.
	// Most of the time it doesn't need to loop.
	getEventually := func(
		t *testing.T, args []string, assertOut func(t assert.TestingT, stdout string)) bool {
		return assert.EventuallyWithT(t,
			func(collect *assert.CollectT) {
				stdout := runSubCAResourceCommand(t, authClient, args...)
				assertOut(collect, stdout)
			},
			5*time.Second,
			10*time.Millisecond,
		)
	}

	if !t.Run("create", func(t *testing.T) {
		runSubCAResourceCommand(t, authClient, "create", caOverridePath)

		wantName := "name: " + clusterName + "\n"
		getEventually(t,
			[]string{"get", "ca_overrides"},
			func(t assert.TestingT, stdout string) {
				assert.Contains(t, stdout, wantName)
			})
	}) {
		t.Skip("tctl create failed, test cannot continue")
	}

	t.Run("get by caType", func(t *testing.T) {
		stdout := runSubCAResourceCommand(t, authClient, "get", "ca_overrides/"+caType)

		wantName := "name: " + clusterName + "\n"
		assert.Contains(t, stdout, wantName)
	})

	t.Run("upsert", func(t *testing.T) {
		newData := `
kind: cert_authority_override
sub_kind: ` + caType + `
version: v1
metadata:
  name: ` + clusterName + `
  description: upsert-test
spec: {}
`
		require.NoError(t, os.WriteFile(caOverridePath, []byte(newData), 0644))

		runSubCAResourceCommand(t, authClient, "create", "-f", caOverridePath)

		wantDesc := "description: upsert-test\n"
		getEventually(t,
			[]string{"get", "ca_overrides"},
			func(t assert.TestingT, stdout string) {
				assert.Contains(t, stdout, wantDesc)
			})
	})

	t.Run("delete", func(t *testing.T) {
		runSubCAResourceCommand(t, authClient, "rm", "ca_overrides/"+caType)

		getEventually(t,
			[]string{"get", "ca_overrides", "--format=json"},
			func(t assert.TestingT, stdout string) {
				assert.Equal(t, wantEmptyList, stdout)
			})
	})
}

func runSubCAResourceCommand(t *testing.T, authClient *authclient.Client, args ...string) (stdout string) {
	out := &bytes.Buffer{}
	require.NoError(t,
		runCommand(t, authClient, &common.ResourceCommand{Stdout: out}, args),
	)
	return out.String()
}

func setupSubCASuite(t *testing.T) *authclient.Client {
	t.Helper()

	process, err := testenv.NewTeleportProcess(
		t.TempDir(),
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			testModules := &modulestest.Modules{
				TestBuildType: modules.BuildEnterprise,
			}
			cfg.PluginRegistry = plugin.NewRegistry()
			cfg.Modules = testModules
			authPlugin, err := authe.NewPlugin(authe.Config{
				License:        authe.ValidLicense{},
				LicenseChecker: authe.ValidLicense{},
				Modules:        testModules,
			})
			require.NoError(t, err)
			require.NoError(t, cfg.PluginRegistry.Add(authPlugin))
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})

	caAdminRole, err := types.NewRole("ca-admin", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindCertAuthority, services.RW()),
				types.NewRule(types.KindCertAuthorityOverride, services.RW()),
			},
		},
	})
	require.NoError(t, err)
	_, err = process.GetAuthServer().CreateRole(t.Context(), caAdminRole)
	require.NoError(t, err)

	user, err := types.NewUser("admin")
	require.NoError(t, err)
	user.SetRoles([]string{
		teleport.PresetEditorRoleName, // not strictly necessary for Sub CA tests
		caAdminRole.GetName(),
	})
	_, err = process.GetAuthServer().CreateUser(t.Context(), user)
	require.NoError(t, err)

	return makeClient(t, process, user.GetName())
}
