package accessrequests

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/lib/utils/aws"
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

func TestAccessRequestWithSSHResourceConstraints(t *testing.T) {
	sut := common.InitSUT(t,
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithRole(t, "ssh-access", func(r *types.RoleV6) {
			r.Spec.Allow.NodeLabels = types.Labels{types.Wildcard: []string{types.Wildcard}}
			r.Spec.Allow.Logins = []string{"root", "ubuntu"}
		}),
		common.WithRole(t, "requester", func(r *types.RoleV6) {
			r.Spec.Allow.Request = &types.AccessRequestConditions{
				SearchAsRoles: []string{"access", "ssh-access"},
			}
		}),
		common.WithRole(t, "reviewer", func(r *types.RoleV6) {
			r.Spec.Allow.ReviewRequests = &types.AccessReviewConditions{
				Roles:          []string{"access", "ssh-access"},
				PreviewAsRoles: []string{"access", "ssh-access"},
			}
		}),
		common.WithUser(t, "alice", "reviewer"),
		common.WithUser(t, "bob", "requester"),
	)

	auth := sut.Teleport.Process.GetAuthServer()
	mustCreateNode(t.Context(), t, auth, "test-node", "test-node.example.com",
		common.WithNodeLabel("env", "staging"))

	bobWebClient := sut.CreateWebClientForUser(t, "bob")
	aliceWebClient := sut.CreateWebClientForUser(t, "alice")

	// Bob initially has no access to resources
	resources := common.MustListUnifedResources(t, bobWebClient)
	require.Empty(t, resources.Items)

	// When searching as role, should see the node with both logins as requestable
	resourcesRequestable := common.MustListUnifedResources(t, bobWebClient, common.WithSearchAsRole(), common.WithIncludeRequestable())
	require.Len(t, resourcesRequestable.Items, 1)
	require.True(t, resourcesRequestable.Items[0].RequiresRequest)
	require.ElementsMatch(t, resourcesRequestable.Items[0].SSHLogins, []string{"root", "ubuntu"})

	// Bob creates an access request for the node, with only the "ubuntu" login
	accessRequest, err := common.CreateAccessRequest(t.Context(), bobWebClient, ui.AccessRequestParameters{
		Roles:       []string{"ssh-access"},
		RequestKind: types.AccessRequestKind_SHORT_TERM,
		ResourceAccessIDs: []ui.ResourceAccessID{
			{
				ID: ui.ResourceID{
					Kind:        types.KindNode,
					Name:        "test-node",
					ClusterName: "local-site",
				},
				Constraints: &types.ResourceConstraints{
					Details: &types.ResourceConstraints_Ssh{
						Ssh: &types.SSHResourceConstraints{
							Logins: []string{"ubuntu"},
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, accessRequest.ID)

	// Alice approves Bob's request
	common.MustApproveAccessRequest(t, aliceWebClient, accessRequest.ID)

	// Bob assumes the approved access request to gain JIT access
	bobWebJITClient, err := common.AssumeAccessRequestWebClient(t.Context(), bobWebClient, accessRequest.ID)
	require.NoError(t, err)

	// Bob now has access to the node, with only the "ubuntu" login
	resources = common.MustListUnifedResources(t, bobWebJITClient)
	require.Len(t, resources.Items, 1)

	nodeItem := resources.Items[0]
	require.Equal(t, "node", nodeItem.Kind)
	require.Equal(t, []string{"ubuntu"}, nodeItem.SSHLogins)
}

func TestAccessRequestWithResourceConstraints(t *testing.T) {
	sut := common.InitSUT(t,
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithApp("aws-console", "https://console.aws.amazon.com/", map[string]string{"env": "staging"}),
		common.WithRole(t, "aws-access", func(r *types.RoleV6) {
			r.Spec.Allow.AppLabels = types.Labels{types.Wildcard: []string{types.Wildcard}}
			r.Spec.Allow.AWSRoleARNs = []string{"arn:aws:iam::123456789012:role/ReadOnly", "arn:aws:iam::123456789012:role/Admin"}
		}),
		common.WithRole(t, "requester", func(r *types.RoleV6) {
			r.Spec.Allow.Request = &types.AccessRequestConditions{
				SearchAsRoles: []string{"access", "aws-access"},
			}
		}),
		common.WithRole(t, "reviewer", func(r *types.RoleV6) {
			r.Spec.Allow.ReviewRequests = &types.AccessReviewConditions{
				Roles:          []string{"access", "aws-access"},
				PreviewAsRoles: []string{"access", "aws-access"},
			}
		}),
		common.WithUser(t, "alice", "reviewer"),
		common.WithUser(t, "bob", "requester"),
	)

	bobWebClient := sut.CreateWebClientForUser(t, "bob")
	aliceWebClient := sut.CreateWebClientForUser(t, "alice")

	// Bob initially has no access to resources
	resources := common.MustListUnifedResources(t, bobWebClient)
	require.Empty(t, resources.Items)

	// When searchingAsRole, should see both ARNs present
	resourcesRequestable := common.MustListUnifedResources(t, bobWebClient, common.WithSearchAsRole(), common.WithIncludeRequestable())
	require.Len(t, resourcesRequestable.Items, 1)
	require.True(t, resourcesRequestable.Items[0].RequiresRequest)
	require.ElementsMatch(t, resourcesRequestable.Items[0].AWSRoles, []aws.Role{
		{
			Name:            "ReadOnly",
			Display:         "ReadOnly",
			AccountID:       "123456789012",
			ARN:             "arn:aws:iam::123456789012:role/ReadOnly",
			RequiresRequest: true,
		},
		{
			Name:            "Admin",
			Display:         "Admin",
			AccountID:       "123456789012",
			ARN:             "arn:aws:iam::123456789012:role/Admin",
			RequiresRequest: true,
		},
	})

	// Bob creates an access request for the dev app, with a specific AWS Role ARN
	accessRequest, err := common.CreateAccessRequest(t.Context(), bobWebClient, ui.AccessRequestParameters{
		Roles:       []string{"aws-access"},
		RequestKind: types.AccessRequestKind_SHORT_TERM,
		ResourceAccessIDs: []ui.ResourceAccessID{
			{
				ID: ui.ResourceID{
					Kind:        types.KindApp,
					Name:        "aws-console",
					ClusterName: "local-site",
				},
				Constraints: &types.ResourceConstraints{
					Details: &types.ResourceConstraints_AwsConsole{
						AwsConsole: &types.AWSConsoleResourceConstraints{
							RoleArns: []string{"arn:aws:iam::123456789012:role/ReadOnly"},
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, accessRequest.ID)

	// Alice approves Bob's request
	common.MustApproveAccessRequest(t, aliceWebClient, accessRequest.ID)

	// Bob assumes the approved access request to gain JIT access
	bobWebJITClient, err := common.AssumeAccessRequestWebClient(t.Context(), bobWebClient, accessRequest.ID)
	require.NoError(t, err)

	// Bob now has access to the aws console app, with only the ReadOnly ARN
	resources = common.MustListUnifedResources(t, bobWebJITClient)
	require.Len(t, resources.Items, 1)

	awsConsoleItem := resources.Items[0]
	awsRoles := awsConsoleItem.AWSRoles
	require.Len(t, awsRoles, 1)
	require.Equal(t, aws.Role{
		Name:            "ReadOnly",
		Display:         "ReadOnly",
		AccountID:       "123456789012",
		ARN:             "arn:aws:iam::123456789012:role/ReadOnly",
		RequiresRequest: false,
	}, awsRoles[0])
}
