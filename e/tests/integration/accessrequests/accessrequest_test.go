package accessrequests

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/e/tests/common"
)

func TestAccessRequest(t *testing.T) {
	sut := common.InitSUT(t,
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice", "reviewer"),
		common.WithUser(t, "bob", "requester"),
		common.WithApp("dev", "http://localhost:443", map[string]string{"env": "dev"}),
	)

	bobWebClient := sut.CreateWebClientForUser(t, "bob")
	aliceWebClient := sut.CreateWebClientForUser(t, "alice")

	// Bob initially has no access to resources
	resources := common.MustListUnifedResources(t, bobWebClient)
	require.Empty(t, resources.Items)

	// Bob creates an access request for the dev app
	accessRequest, err := common.CreateAccessRequest(t.Context(), bobWebClient, ui.AccessRequestParameters{
		Roles:       []string{"access"},
		RequestKind: types.AccessRequestKind_SHORT_TERM,
		ResourceIDs: []ui.ResourceID{
			{
				Kind:        types.KindApp,
				Name:        "dev",
				ClusterName: "local-site",
			},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, accessRequest.ID)

	// Alice approves Bob's access request
	common.MustApproveAccessRequest(t, aliceWebClient, accessRequest.ID)

	// Bob assumes the approved access request to gain JIT access
	bobWebJITClient, err := common.AssumeAccessRequestWebClient(t.Context(), bobWebClient, accessRequest.ID)
	require.NoError(t, err)

	// Bob now has access to the dev app
	resources = common.MustListUnifedResources(t, bobWebJITClient)
	require.Len(t, resources.Items, 1)
}
