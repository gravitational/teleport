package integration

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/stretchr/testify/require"
)

// TestSignup sets up a test instance of Teleport and runs a playwright test against it to test the signup flow.
func TestSignup(t *testing.T) {
	rc, ctx := helpers.CreateTeleportTestInstance(t)

	as := rc.Process.GetAuthServer()

	accessRole := services.NewPresetAccessRole()

	// Create a test user.
	testUser, err := types.NewUser("testuser")
	require.NoError(t, err)
	testUser.SetRoles([]string{accessRole.GetName()})
	user, err := as.UpsertUser(ctx, testUser)

	inviteToken, err := as.CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
		Name: user.GetName(),
	})
	require.NoError(t, err)

	// Generate the URL the playwright test will start from.
	startUrl := fmt.Sprintf("START_URL=https://%s/web/invite/%s", rc.Web, inviteToken.GetName())

	// Start the playwright test
	cmd := exec.Command("pnpm", "test-e2e", "signup.spec.ts")
	cmd.Env = append(os.Environ(), startUrl)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())
}
