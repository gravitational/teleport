package okta

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/events"
)

func Test_UserSync_preserves_SCIMAttrLabel(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	fakeOkta := newFakeOktaServer(
		withUserCount(7),
		withAppCount(1),
		withGroupCount(1),
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)

	scimToken := createAndWaitForOktaIntegration(t, sut, fakeOkta)
	scimClient := createSCIMClient(t, sut, scimToken)

	const extraSCIMAttr, extraSCIMAttrVal = "test_scim_attr", "test_scim_value"

	// Create a watcher so that we can detect when our new user lands in the auth cache.
	userWatcher := sut.NewResourceWatcher(t, types.KindUser)

	aliceOkta := fakeOkta.CreateUser("alice")
	require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, aliceOkta.Id))
	aliceSCIM := &scimsdk.User{
		ExternalID: aliceOkta.Id,
		UserName:   oktaUserLogin(aliceOkta),
		Active:     true,
		Attributes: map[string]any{extraSCIMAttr: extraSCIMAttrVal},
	}

	// First let's create the user with SCIM.
	_, err := scimClient.CreateUser(ctx, aliceSCIM)
	require.NoError(t, err)

	// Wait for the SCIM user to be propagated to the cache.
	waitForResource(t, userWatcher, func(u types.User) bool {
		return u.GetName() == aliceSCIM.UserName
	})

	// Enable Okta user sync and wait for the sync to happen.
	start := time.Now()
	mustUpdateOktaIntegration(ctx, t, sut.GetOktaAuthClient(t, "alice-admin"), oktav1.UpdateIntegrationRequest_builder{
		EnableUserSync: true,
	}.Build())
	mustWaitForEvent(t, sut, events.OktaUserSyncEvent, withTimePoint(start))

	// Wait for the user update written by the sync to reach the cache before
	// asserting on it. (The Okta user status label is set by the Okta sync
	// but not by SCIM provisioning, so this guarantees we observe the post-sync resource).
	waitForResource(t, userWatcher, func(u types.User) bool {
		if u.GetName() != aliceSCIM.UserName {
			return false
		}
		_, ok := u.GetLabel(eteleport.OktaUserStatusLabel)
		return ok
	})

	// Check if the user has preserved SCIM attributes.
	alice, err := sut.Teleport.Process.GetAuthServer().GetUser(ctx, aliceSCIM.UserName, false)
	require.NoError(t, err)
	require.NotEmpty(t, alice.GetStaticLabels()[eteleport.SCIMAttrsLabel])

	aliceSCIM, err = scimClient.GetUser(ctx, aliceSCIM.UserName)
	require.NoError(t, err)
	require.Equal(t, extraSCIMAttrVal, aliceSCIM.Attributes[extraSCIMAttr])
}
