//go:build webassets_embed

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

package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// TestSignup sets up a test instance of Teleport and runs a playwright test against it to test the signup flow.
func TestSignup(t *testing.T) {
	rc, ctx := createTeleportTestInstanceForWebTests(t)

	as := rc.Process.GetAuthServer()

	accessRole := services.NewPresetAccessRole()
	editorRole := services.NewPresetEditorRole()

	// Create a test user.
	testUser, err := types.NewUser("testuser")
	require.NoError(t, err)
	testUser.SetRoles([]string{accessRole.GetName(), editorRole.GetName()})
	user, err := as.UpsertUser(ctx, testUser)
	require.NoError(t, err)

	inviteToken, err := as.CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
		Name: user.GetName(),
	})
	require.NoError(t, err)

	// Generate the URL with the invite link
	startUrl := fmt.Sprintf("START_URL=https://%s/web/invite/%s", rc.Web, inviteToken.GetName())

	// Start the playwright test using the "signup" project which skips the auth setup.
	cmd := exec.Command("pnpm", "test", "--project=signup", "signup.spec.ts")
	cmd.Env = append(os.Environ(), startUrl)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())
}

// TestRoleManagement sets up a test instance of Teleport user to test role management in the UI.
func TestRoleManagement(t *testing.T) {
	makeBootstrappedSetupAndRunTest(t, "roles.spec.ts")
}

// TestAuthConnectorManagement sets up a test instance of Teleport user to test auth connector management in the UI.
func TestAuthConnectorManagement(t *testing.T) {
	makeBootstrappedSetupAndRunTest(t, "authconnectors.spec.ts")
}

// makeBootstrappedSetupAndRunTest sets up a test instance bootstrapped with state.yaml
// and runs a playwright test. The bootstrapped state includes a pre-configured user "bob" with password "secret"
func makeBootstrappedSetupAndRunTest(t *testing.T, playwrightTest string) {
	bootstrapResources := readBootstrapResources(t)
	rc, _ := createTeleportTestInstanceForWebTests(t, bootstrapResources...)

	startUrl := fmt.Sprintf("START_URL=https://%s/web/login", rc.Web)

	cmd := exec.Command("pnpm", "test", "--project=setup", playwrightTest)
	cmd.Env = append(os.Environ(), startUrl)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())
}

func readBootstrapResources(t *testing.T) []types.Resource {
	stateFile := filepath.Join("config", "state.yaml")
	resources, err := config.ReadResources(stateFile)
	require.NoError(t, err)
	require.NotEmpty(t, resources)

	var filtered []types.Resource
	for _, r := range resources {
		filtered = append(filtered, r)
	}

	require.NotEmpty(t, filtered, "state.yaml must contain at least one resource")
	return filtered
}

// createTeleportTestInstanceForWebTests creates a new Teleport instance to be used for Web UI e2e tests.
// Optional bootstrapResources are applied on first startup to pre-populate the instance with resources.
func createTeleportTestInstanceForWebTests(t *testing.T, bootstrapResources ...types.Resource) (instance *helpers.TeleInstance, ctx context.Context) {
	privateKey, publicKey, err := testauthority.GenerateKeyPair()
	require.NoError(t, err)

	cfg := helpers.InstanceConfig{
		ClusterName: "teleport-e2e",
		HostID:      uuid.New().String(),
		NodeName:    helpers.Host,
		Logger:      logtest.NewLogger(),
		Priv:        privateKey,
		Pub:         publicKey,
	}
	cfg.Listeners = helpers.SingleProxyPortSetupOn(helpers.Host)(t, &cfg.Fds)
	rc := helpers.NewInstance(t, cfg)

	rcConf := servicecfg.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Auth.Preference.SetSecondFactor("on")
	rcConf.Auth.Preference.SetWebauthn(&types.Webauthn{RPID: helpers.Host})
	rcConf.Proxy.Enabled = true
	rcConf.SSH.Enabled = false
	rcConf.Proxy.DisableWebInterface = false
	rcConf.Version = "v3"
	rcConf.Auth.BootstrapResources = bootstrapResources

	ctx, contextCancel := context.WithCancel(context.Background())

	err = rc.CreateEx(t, nil, rcConf)
	require.NoError(t, err)
	err = rc.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, rc.StopAll())
		contextCancel()
	})

	return rc, ctx
}
