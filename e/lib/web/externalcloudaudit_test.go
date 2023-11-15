package web

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/externalcloudaudit"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/modules"
)

func TestGenerateDraftExternalCloudAudit(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
		},
	})

	ctx := context.Background()
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	clusterName := s.testAuthServer.ClusterName()

	// Precondition: there must be a cluster audit config with a region
	auditConfig, err := s.testAuthServer.Auth().GetClusterAuditConfig(ctx)
	require.NoError(t, err)
	auditConfig.SetRegion("us-west-2")
	require.NoError(t, s.testAuthServer.Auth().SetClusterAuditConfig(ctx, auditConfig))

	generateEndpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "integration", "externalcloudaudit", "generate")
	resp, err := webPack.clt.PostJSON(ctx, generateEndpoint, ui.GenerateDraftExternalCloudAuditRequest{
		IntegrationName: "test-integration",
	})
	require.NoError(t, err)

	// Make sure we can decode the generated config and it's valid
	var generated externalcloudaudit.ExternalCloudAudit
	require.NoError(t, json.NewDecoder(resp.Reader()).Decode(&generated))
	require.NoError(t, generated.CheckAndSetDefaults())

	// Make sure an unauthenticated client can't generate
	publicClt := s.client(t)
	_, err = publicClt.PostJSON(ctx, generateEndpoint, ui.GenerateDraftExternalCloudAuditRequest{
		IntegrationName: "test-integration",
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func TestBuildExternalCloudAuditBootstrapScript(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
		},
	})

	ctx := context.Background()
	s := newWebSuite(t)

	publicClt := s.client(t)
	scriptEndpoint := publicClt.Endpoint(
		"webapi",
		"scripts",
		"integration",
		"externalcloudaudit-bootstrap.sh",
	)

	for _, tc := range []struct {
		desc        string
		params      url.Values
		errContains []string
		expectArgs  string
	}{
		{
			desc: "pass",
			params: url.Values{
				"role":       {"test-iam-role"},
				"region":     {"us-west-2"},
				"policy":     {"test-policy"},
				"recordings": {"s3://teleport-longterm-test/recordings"},
				"events":     {"s3://teleport-longterm-test/events"},
				"results":    {"s3://teleport-transient-test/results"},
				"workgroup":  {"teleport_events_test"},
				"db":         {"teleport_events_test"},
				"table":      {"teleport_events_test"},
			},
			expectArgs: `integration configure externalcloudaudit ` +
				`--bootstrap --aws-region=us-west-2 ` +
				`--role=test-iam-role --policy=test-policy ` +
				`--session-recordings=s3://teleport-longterm-test/recordings ` +
				`--audit-events=s3://teleport-longterm-test/events ` +
				`--athena-results=s3://teleport-transient-test/results ` +
				`--athena-workgroup=teleport_events_test ` +
				`--glue-database=teleport_events_test ` +
				`--glue-table=teleport_events_test`,
		},
		{
			desc: "missing role",
			params: url.Values{
				"region":     {"us-west-2"},
				"policy":     {"test-policy"},
				"recordings": {"s3://teleport-longterm-test/recordings"},
				"events":     {"s3://teleport-longterm-test/events"},
				"results":    {"s3://teleport-transient-test/results"},
				"workgroup":  {"teleport_events_test"},
				"db":         {"teleport_events_test"},
				"table":      {"teleport_events_test"},
			},
			errContains: []string{
				`required parameter "role" not found`,
			},
		},
		{
			desc: "bad region",
			params: url.Values{
				"role":       {"test-iam-role"},
				"region":     {"notaregion!"},
				"policy":     {"test-policy"},
				"recordings": {"s3://teleport-longterm-test/recordings"},
				"events":     {"s3://teleport-longterm-test/events"},
				"results":    {"s3://teleport-transient-test/results"},
				"workgroup":  {"teleport_events_test"},
				"db":         {"teleport_events_test"},
				"table":      {"teleport_events_test"},
			},
			errContains: []string{
				`validating region param`,
				`"notaregion!" is invalid`,
			},
		},
		{
			desc: "bad uri",
			params: url.Values{
				"role":       {"test-iam-role"},
				"region":     {"us-east-2"},
				"policy":     {"test-policy"},
				"recordings": {"s3://teleport_longterm_test/recordings"},
				"events":     {"s3://teleport-longterm-test/events"},
				"results":    {"s3://teleport-transient-test/results"},
				"workgroup":  {"teleport_events_test"},
				"db":         {"teleport_events_test"},
				"table":      {"teleport_events_test"},
			},
			errContains: []string{
				`validating recordings param`,
				`bucket name "teleport_longterm_test" includes illegal special character`,
			},
		},
		{
			desc: "bad glue table",
			params: url.Values{
				"role":       {"test-iam-role"},
				"region":     {"us-east-2"},
				"policy":     {"test-policy"},
				"recordings": {"s3://teleport-longterm-test/recordings"},
				"events":     {"s3://teleport-longterm-test/events"},
				"results":    {"s3://teleport-transient-test/results"},
				"workgroup":  {"teleport_events_test"},
				"db":         {"teleport_events_test"},
				"table":      {"teleport-events-test"},
			},
			errContains: []string{
				`validating table param`,
				`glue resource name "teleport-events-test" is invalid`,
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			resp, err := publicClt.Get(ctx, scriptEndpoint, tc.params)
			if len(tc.errContains) > 0 {
				for _, msg := range tc.errContains {
					require.ErrorContains(t, err, msg)
				}
				return
			}
			require.NoError(t, err)
			script := string(resp.Bytes())
			require.Contains(t, script, tc.expectArgs)
		})
	}
}

func TestExternalCloudAuditPromote(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
		},
	})

	ctx := context.Background()
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	clusterName := s.testAuthServer.ClusterName()

	// assert that it fails if no drafts exist
	promoteEndpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "integration", "externalcloudaudit", "promote")
	_, err := webPack.clt.PostJSON(ctx, promoteEndpoint, nil)
	require.Error(t, err)
	require.False(t, trace.IsAccessDenied(err))

	// create draft
	client := s.newAdminAuthClient(ctx, t).ExternalCloudAuditClient()
	_, err = client.GenerateDraftExternalCloudAudit(ctx, "test-integration", "us-west-2")
	require.NoError(t, err)
	// assert that we can promote it
	_, err = webPack.clt.PostJSON(ctx, promoteEndpoint, nil)
	require.NoError(t, err)

	// Make sure an unauthenticated client can't promote
	publicClt := s.client(t)
	_, err = publicClt.PostJSON(ctx, promoteEndpoint, nil)
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func TestExternalCloudAuditGetCluster(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
		},
	})

	ctx := context.Background()
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	clusterName := s.testAuthServer.ClusterName()

	getClusterEndpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "integration", "externalcloudaudit", "cluster")

	// assert that it returns a not found error if no active cluster audit exist
	_, err := webPack.clt.Get(ctx, getClusterEndpoint, nil)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	// assert that it returns the existing active cluster audit
	client := s.newAdminAuthClient(ctx, t).ExternalCloudAuditClient()
	_, err = client.GenerateDraftExternalCloudAudit(ctx, "test-integration", "us-west-2")
	require.NoError(t, err)
	err = client.PromoteToClusterExternalCloudAudit(ctx)
	require.NoError(t, err)

	resp, err := webPack.clt.Get(ctx, getClusterEndpoint, nil)
	require.NoError(t, err)
	// Make sure we can decode the generated config and it's valid
	var returned ui.ExternalCloudAudit
	require.NoError(t, json.NewDecoder(resp.Reader()).Decode(&returned))
	require.Equal(t, "test-integration", returned.IntegrationName)

	// Make sure an unauthenticated client can't get active cluster audit
	publicClt := s.client(t)
	_, err = publicClt.Get(ctx, getClusterEndpoint, nil)
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func TestExternalCloudAuditGetDraft(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
		},
	})

	ctx := context.Background()
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	clusterName := s.testAuthServer.ClusterName()

	getDraftEndpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "integration", "externalcloudaudit", "draft")

	// assert that it returns a not found error if no draft cluster audit exist
	_, err := webPack.clt.Get(ctx, getDraftEndpoint, nil)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	// assert that it returns the existing draft cluster audit
	client := s.newAdminAuthClient(ctx, t).ExternalCloudAuditClient()
	_, err = client.GenerateDraftExternalCloudAudit(ctx, "test-integration", "us-west-2")
	require.NoError(t, err)

	resp, err := webPack.clt.Get(ctx, getDraftEndpoint, nil)
	require.NoError(t, err)
	// Make sure we can decode the generated config and it's valid
	var returned ui.ExternalCloudAudit
	require.NoError(t, json.NewDecoder(resp.Reader()).Decode(&returned))
	require.Equal(t, "test-integration", returned.IntegrationName)

	// Make sure an unauthenticated client can't get draft cluster audit
	publicClt := s.client(t)
	_, err = publicClt.Get(ctx, getDraftEndpoint, nil)
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func TestExternalCloudAuditDeleteCluster(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
		},
	})

	ctx := context.Background()
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	clusterName := s.testAuthServer.ClusterName()

	deleteClusterEndpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "integration", "externalcloudaudit", "cluster")

	// assert that it returns a not found error if no active cluster audit exist
	_, err := webPack.clt.Delete(ctx, deleteClusterEndpoint)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	// assert that it deletes the existing active cluster audit
	client := s.newAdminAuthClient(ctx, t).ExternalCloudAuditClient()
	_, err = client.GenerateDraftExternalCloudAudit(ctx, "test-integration", "us-west-2")
	require.NoError(t, err)
	err = client.PromoteToClusterExternalCloudAudit(ctx)
	require.NoError(t, err)

	_, err = webPack.clt.Delete(ctx, deleteClusterEndpoint)
	require.NoError(t, err)

	_, err = client.GetClusterExternalCloudAudit(ctx)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	// Make sure an unauthenticated client can't delete active cluster audit
	publicClt := s.client(t)
	_, err = publicClt.Delete(ctx, deleteClusterEndpoint)
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func TestExternalCloudAuditDeleteDraft(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
		},
	})

	ctx := context.Background()
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	clusterName := s.testAuthServer.ClusterName()

	deleteDraftEndpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "integration", "externalcloudaudit", "draft")

	// assert that it returns a not found error if no draft cluster audit exist
	_, err := webPack.clt.Delete(ctx, deleteDraftEndpoint)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	// assert that it deletes the existing draft cluster audit
	client := s.newAdminAuthClient(ctx, t).ExternalCloudAuditClient()
	_, err = client.GenerateDraftExternalCloudAudit(ctx, "test-integration", "us-west-2")
	require.NoError(t, err)

	_, err = webPack.clt.Delete(ctx, deleteDraftEndpoint)
	require.NoError(t, err)

	_, err = client.GetDraftExternalCloudAudit(ctx)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	// Make sure an unauthenticated client can't delete draft cluster audit
	publicClt := s.client(t)
	_, err = publicClt.Delete(ctx, deleteDraftEndpoint)
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}
