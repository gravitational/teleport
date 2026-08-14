package accessrequests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	componentfeaturesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/componentfeatures/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
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

// TestAccessRequestSuggestedAccessLists exercises the owner-reviewer Access List
// suggestion flow.
func TestAccessRequestSuggestedAccessLists(t *testing.T) {
	sut := common.InitSUT(t,
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithApp("dev", "http://localhost:443", map[string]string{"env": "dev"}),
		common.WithRole(t, "requester", func(r *types.RoleV6) {
			r.Spec.Allow.Request = &types.AccessRequestConditions{
				SearchAsRoles: []string{"access"},
			}
		}),
		// The reviewer role grants no access_list read, so the reviewer can only
		// reach the candidate Access List through inherited ownership, not RBAC.
		common.WithRole(t, "reviewer", func(r *types.RoleV6) {
			r.Spec.Allow.ReviewRequests = &types.AccessReviewConditions{
				Roles: []string{"access"},
			}
		}),
		common.WithUser(t, "alice", "reviewer"),
		common.WithUser(t, "bob", "requester"),
	)

	// reviewers-acl: alice is a member, not a direct owner.
	common.CreateAccessList(t, sut,
		common.WithName("reviewers-acl"),
		common.WithOwners("list-owner"),
		common.WithMembers("alice"),
	)

	// app-acl grants access to the dev app and is owned by the nested reviewers-acl,
	// so alice is only an inherited owner of app-acl.
	common.CreateAccessList(t, sut,
		common.WithName("app-acl"),
		common.WithListOwners("reviewers-acl"),
		common.WithGrants(accesslist.Grants{Roles: []string{"access"}}),
	)

	// other-acl also grants the dev app but is owned by someone else, so alice can
	// neither own nor read it. It must be skipped from suggestions, not fail them.
	common.CreateAccessList(t, sut,
		common.WithName("other-acl"),
		common.WithOwners("list-owner"),
		common.WithGrants(accesslist.Grants{Roles: []string{"access"}}),
	)

	bobWebClient := sut.CreateWebClientForUser(t, "bob")
	aliceWebClient := sut.CreateWebClientForUser(t, "alice")

	// Bob requests access to the dev app.
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

	// suggestedNames returns the Access Lists suggested to alice for the request.
	// The fetch must succeed even though other-acl is an unreadable candidate.
	suggestedNames := func(t *testing.T) []string {
		t.Helper()
		suggestions, err := common.GetSuggestedAccessLists(t.Context(), aliceWebClient, accessRequest.ID)
		require.NoError(t, err)
		var names []string
		for _, al := range suggestions.AccessLists {
			names = append(names, al.GetName())
		}
		return names
	}

	// app-acl is suggested even though alice's ownership is inherited via the nested
	// reviewers-acl, resolved server-side and trusted (she can't enumerate the
	// nested list's members herself).
	t.Run("inherited ownership is honored", func(t *testing.T) {
		require.Contains(t, suggestedNames(t), "app-acl")
	})

	// other-acl, a valid promotion candidate alice can't read, is skipped rather
	// than failing the whole suggestions request.
	t.Run("skips inaccessible candidates", func(t *testing.T) {
		names := suggestedNames(t)
		require.Contains(t, names, "app-acl")
		require.NotContains(t, names, "other-acl")
	})
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

// TestAgentlessAppServerResourceConstraints verifies that agentless app servers
// (created statically with integration set and no ComponentFeatures) have features
// computed at read time, and that the full access request flow works for them.
func TestAgentlessAppServerResourceConstraints(t *testing.T) {
	sut := common.InitSUT(t,
		common.WithLicense("../../../fixtures/license-eub.pem"),
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

	auth := sut.Teleport.Process.GetAuthServer()

	// Create an agentless AWS Console app server: integration set, no ComponentFeatures.
	// This simulates the OIDC integration path or tctl/gRPC creation.
	appServer, err := types.NewAppServerV3(types.Metadata{
		Name: "aws-console-agentless",
	}, types.AppServerSpecV3{
		HostID: "fake-proxy-id",
		App: &types.AppV3{
			Metadata: types.Metadata{
				Name: "aws-console-agentless",
			},
			Spec: types.AppSpecV3{
				URI:         constants.AWSConsoleURL,
				Cloud:       "AWS",
				Integration: "test-integration",
			},
		},
	})
	require.NoError(t, err)
	// Explicitly do not set ComponentFeatures.
	_, err = auth.UpsertApplicationServer(t.Context(), appServer)
	require.NoError(t, err)

	// Create an otherwise-identical AWS Console app server with no integration.
	// Without an integration it is not "agentless" (it would be served by an app
	// agent), so its features are read from the stored spec instead of computed.
	// With no ComponentFeatures set, it must not advertise ResourceConstraintsV1.
	noIntegrationAppServer, err := types.NewAppServerV3(types.Metadata{
		Name: "aws-console-no-integration",
	}, types.AppServerSpecV3{
		HostID: "fake-proxy-id",
		App: &types.AppV3{
			Metadata: types.Metadata{
				Name: "aws-console-no-integration",
			},
			Spec: types.AppSpecV3{
				URI:   constants.AWSConsoleURL,
				Cloud: "AWS",
			},
		},
	})
	require.NoError(t, err)
	_, err = auth.UpsertApplicationServer(t.Context(), noIntegrationAppServer)
	require.NoError(t, err)

	bobWebClient := sut.CreateWebClientForUser(t, "bob")
	aliceWebClient := sut.CreateWebClientForUser(t, "alice")

	// When searching as role, both apps appear as requestable. Only the agentless
	// app (integration set, no ComponentFeatures) has ResourceConstraintsV1 computed
	// at read time; the otherwise-identical app with no integration is treated as
	// agent-backed, so with no ComponentFeatures it advertises no features.
	//
	// Retried because end-to-end feature support also depends on the cluster's Auth
	// and Proxy ComponentFeatures having propagated to the Proxy cache.
	const resourceConstraintsV1 = int(componentfeaturesv1.ComponentFeatureID_COMPONENT_FEATURE_ID_RESOURCE_CONSTRAINTS_V1)
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		resourcesRequestable := common.MustListUnifedResources(t, bobWebClient, common.WithSearchAsRole(), common.WithIncludeRequestable())
		if !assert.Len(collect, resourcesRequestable.Items, 2) {
			return
		}

		for _, item := range resourcesRequestable.Items {
			assert.Equal(collect, "app", item.Kind)
			assert.True(collect, item.RequiresRequest)
			switch item.Name {
			case "aws-console-agentless":
				assert.Contains(collect, item.SupportedFeatureIDs, resourceConstraintsV1)
			case "aws-console-no-integration":
				assert.NotContains(collect, item.SupportedFeatureIDs, resourceConstraintsV1)
			default:
				assert.Failf(collect, "unexpected resource in list", "name %q", item.Name)
			}
		}
	}, 10*time.Second, 50*time.Millisecond)

	// Bob creates an access request with a specific AWS Role ARN constraint
	accessRequest, err := common.CreateAccessRequest(t.Context(), bobWebClient, ui.AccessRequestParameters{
		Roles:       []string{"aws-access"},
		RequestKind: types.AccessRequestKind_SHORT_TERM,
		ResourceAccessIDs: []ui.ResourceAccessID{
			{
				ID: ui.ResourceID{
					Kind:        types.KindApp,
					Name:        "aws-console-agentless",
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

	// Alice approves
	common.MustApproveAccessRequest(t, aliceWebClient, accessRequest.ID)

	// Bob assumes the request
	bobWebJITClient, err := common.AssumeAccessRequestWebClient(t.Context(), bobWebClient, accessRequest.ID)
	require.NoError(t, err)

	// Bob sees the app with only the approved ARN
	resources := common.MustListUnifedResources(t, bobWebJITClient)
	require.Len(t, resources.Items, 1)

	awsRoles := resources.Items[0].AWSRoles
	require.Len(t, awsRoles, 1)
	require.Equal(t, "arn:aws:iam::123456789012:role/ReadOnly", awsRoles[0].ARN)
}
