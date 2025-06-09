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
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// TestSignup sets up a test instance of Teleport and runs a playwright test against it to test the signup flow.
func TestSignup(t *testing.T) {
	makeBasicSetupAndRunTest(t, "signup.spec.ts")
}

// TestCreateNewRole sets up a test instance of Teleport and a user to test role management in the UI.
func TestRoleManagement(t *testing.T) {
	makeBasicSetupAndRunTest(t, "roles.spec.ts")
}

// TestAuthConnectorManagement sets up a test instance of Teleport and a user to test auth connector management in the UI.
func TestAuthConnectorManagement(t *testing.T) {
	makeBasicSetupAndRunTest(t, "authconnectors.spec.ts")
}

// makeBasicSetupAndRunTest sets up a test instance of Teleport and a user with the access and editor roles and runs a playwright test.
// This is a helper function in cases where there is no additional backend setup required beyond creating an invite link.
func makeBasicSetupAndRunTest(t *testing.T, playwrightTest string) {
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

	// Generate the URL the playwright test will start from.
	startUrl := fmt.Sprintf("START_URL=https://%s/web/invite/%s", rc.Web, inviteToken.GetName())

	// Start the playwright test
	cmd := exec.Command("pnpm", "test", playwrightTest)
	cmd.Env = append(os.Environ(), startUrl)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())
}

// createTeleportTestInstanceForWebTests creates a new Teleport instance to be used for Web UI e2e tests.
// Using this function requires the `webassets_embed` build tag.
func createTeleportTestInstanceForWebTests(t *testing.T) (instance *helpers.TeleInstance, ctx context.Context) {
	privateKey, publicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	cfg := helpers.InstanceConfig{
		ClusterName: "test-cluster",
		HostID:      uuid.New().String(),
		NodeName:    helpers.Host,
		Logger:      utils.NewSlogLoggerForTests(),
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
