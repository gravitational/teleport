package web

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/externalauditstorage"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/modules"
)

const (
	testAthenaURI       = "athena://db.table?topicArn=arn:aws:sns:eu-central-1:accnr:topicName&queryResultsS3=s3://testbucket/query-result/&workgroup=workgroup&locationS3=s3://testbucket/events-location&queueURL=https://sqs.eu-central-1.amazonaws.com/accnr/sqsname&largeEventsS3=s3://testbucket/largeevents"
	testRegion          = "us-west-2"
	testIntegrationName = "test-integration"
	testIAMRoleARN      = "test-iam-role"
)

func setupPreconditions(t *testing.T, auth *auth.Server) {
	ctx := context.Background()

	// There must be a cluster audit config with a region and athena URI.
	auditConfig, err := auth.GetClusterAuditConfig(ctx)
	require.NoError(t, err)
	auditConfig.SetRegion(testRegion)
	auditConfig.SetAuditEventsURIs([]string{testAthenaURI})
	require.NoError(t, auth.SetClusterAuditConfig(ctx, auditConfig))

	// There must be an AWS OIDC integration.
	oidcIntegration, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: testIntegrationName},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: testIAMRoleARN,
		},
	)
	require.NoError(t, err)
	_, err = auth.CreateIntegration(ctx, oidcIntegration)
	require.NoError(t, err)
}

func TestGenerateDraftExternalAuditStorage(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.ExternalAuditStorage: {Enabled: true},
			},
		},
	})

	ctx := context.Background()
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	clusterName := s.testAuthServer.ClusterName()

	setupPreconditions(t, s.testAuthServer.Auth())

	generateEndpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "integration", "externalauditstorage", "generate")
	resp, err := webPack.clt.PostJSON(ctx, generateEndpoint, ui.GenerateDraftExternalAuditStorageRequest{
		IntegrationName: testIntegrationName,
	})
	require.NoError(t, err)

	// Make sure we can decode the generated config and it's valid
	var generated externalauditstorage.ExternalAuditStorage
	require.NoError(t, json.NewDecoder(resp.Reader()).Decode(&generated))
	require.NoError(t, generated.CheckAndSetDefaults())

	// Make sure an unauthenticated client can't generate
	publicClt := s.client(t)
	_, err = publicClt.PostJSON(ctx, generateEndpoint, ui.GenerateDraftExternalAuditStorageRequest{
		IntegrationName: testIntegrationName,
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func TestBuildExternalAuditStorageBootstrapScript(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.ExternalAuditStorage: {Enabled: true},
			},
		},
	})

	ctx := context.Background()
	s := newWebSuite(t)

	publicClt := s.client(t)
	scriptEndpoint := publicClt.Endpoint(
		"webapi",
		"scripts",
		"integration",
		"externalauditstorage-bootstrap.sh",
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
			expectArgs: `integration configure externalauditstorage ` +
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
		{
			desc: "injection attempt",
			params: url.Values{
				"role":       {"test-iam-role"},
				"region":     {"us-east-2"},
				"policy":     {"test-policy"},
				"recordings": {"s3://test#'cat /etc/passwd;'"},
				"events":     {"s3://teleport-longterm-test/events"},
				"results":    {"s3://teleport-transient-test/results"},
				"workgroup":  {"teleport_events_test"},
				"db":         {"teleport_events_test"},
				"table":      {"teleport_events_test"},
			},
			errContains: []string{
				`formatting session-recordings argument`,
				`Shell Injection Detected`,
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

func TestExternalAuditStoragePromote(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.ExternalAuditStorage: {Enabled: true},
			},
		},
	})

	ctx := context.Background()
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	clusterName := s.testAuthServer.ClusterName()

	setupPreconditions(t, s.testAuthServer.Auth())

	// assert that it fails if no drafts exist
	promoteEndpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "integration", "externalauditstorage", "promote")
	_, err := webPack.clt.PostJSON(ctx, promoteEndpoint, nil)
	require.Error(t, err)
	require.False(t, trace.IsAccessDenied(err))

	// create draft
	client := s.newAdminAuthClient(ctx, t).ExternalAuditStorageClient()
	_, err = client.GenerateDraftExternalAuditStorage(ctx, testIntegrationName, testRegion)
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

func TestExternalAuditStorageGetCluster(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.ExternalAuditStorage: {Enabled: true},
			},
		},
	})

	ctx := context.Background()
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	clusterName := s.testAuthServer.ClusterName()

	setupPreconditions(t, s.testAuthServer.Auth())

	getClusterEndpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "integration", "externalauditstorage", "cluster")

	// assert that it returns a not found error if no active cluster audit exist
	_, err := webPack.clt.Get(ctx, getClusterEndpoint, nil)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	// assert that it returns the existing active cluster audit
	client := s.newAdminAuthClient(ctx, t).ExternalAuditStorageClient()
	_, err = client.GenerateDraftExternalAuditStorage(ctx, testIntegrationName, testRegion)
	require.NoError(t, err)
	err = client.PromoteToClusterExternalAuditStorage(ctx)
	require.NoError(t, err)

	resp, err := webPack.clt.Get(ctx, getClusterEndpoint, nil)
	require.NoError(t, err)
	// Make sure we can decode the generated config and it's valid
	var returned externalauditstorage.ExternalAuditStorage
	require.NoError(t, json.NewDecoder(resp.Reader()).Decode(&returned))
	require.Equal(t, testIntegrationName, returned.Spec.IntegrationName)

	// Make sure an unauthenticated client can't get active cluster audit
	publicClt := s.client(t)
	_, err = publicClt.Get(ctx, getClusterEndpoint, nil)
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func TestExternalAuditStorageGetDraft(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.ExternalAuditStorage: {Enabled: true},
			},
		},
	})

	ctx := context.Background()
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	clusterName := s.testAuthServer.ClusterName()

	setupPreconditions(t, s.testAuthServer.Auth())

	getDraftEndpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "integration", "externalauditstorage", "draft")

	// assert that it returns a not found error if no draft cluster audit exist
	_, err := webPack.clt.Get(ctx, getDraftEndpoint, nil)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	// assert that it returns the existing draft cluster audit
	client := s.newAdminAuthClient(ctx, t).ExternalAuditStorageClient()
	_, err = client.GenerateDraftExternalAuditStorage(ctx, testIntegrationName, testRegion)
	require.NoError(t, err)

	resp, err := webPack.clt.Get(ctx, getDraftEndpoint, nil)
	require.NoError(t, err)
	// Make sure we can decode the generated config and it's valid
	var returned externalauditstorage.ExternalAuditStorage
	require.NoError(t, json.NewDecoder(resp.Reader()).Decode(&returned))
	require.Equal(t, testIntegrationName, returned.Spec.IntegrationName)

	// Make sure an unauthenticated client can't get draft cluster audit
	publicClt := s.client(t)
	_, err = publicClt.Get(ctx, getDraftEndpoint, nil)
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func TestExternalAuditStorageDeleteCluster(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.ExternalAuditStorage: {Enabled: true},
			},
		},
	})

	ctx := context.Background()
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	clusterName := s.testAuthServer.ClusterName()

	setupPreconditions(t, s.testAuthServer.Auth())

	deleteClusterEndpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "integration", "externalauditstorage", "cluster")

	// assert that it returns a not found error if no active cluster audit exist
	_, err := webPack.clt.Delete(ctx, deleteClusterEndpoint)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	// assert that it deletes the existing active cluster audit
	client := s.newAdminAuthClient(ctx, t).ExternalAuditStorageClient()
	_, err = client.GenerateDraftExternalAuditStorage(ctx, testIntegrationName, testRegion)
	require.NoError(t, err)
	err = client.PromoteToClusterExternalAuditStorage(ctx)
	require.NoError(t, err)

	_, err = webPack.clt.Delete(ctx, deleteClusterEndpoint)
	require.NoError(t, err)

	_, err = client.GetClusterExternalAuditStorage(ctx)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	// Make sure an unauthenticated client can't delete active cluster audit
	publicClt := s.client(t)
	_, err = publicClt.Delete(ctx, deleteClusterEndpoint)
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func TestExternalAuditStorageDeleteDraft(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.ExternalAuditStorage: {Enabled: true},
			},
		},
	})

	ctx := context.Background()
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	clusterName := s.testAuthServer.ClusterName()

	setupPreconditions(t, s.testAuthServer.Auth())

	deleteDraftEndpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "integration", "externalauditstorage", "draft")

	// assert that it returns a not found error if no draft cluster audit exist
	_, err := webPack.clt.Delete(ctx, deleteDraftEndpoint)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	// assert that it deletes the existing draft cluster audit
	client := s.newAdminAuthClient(ctx, t).ExternalAuditStorageClient()
	_, err = client.GenerateDraftExternalAuditStorage(ctx, testIntegrationName, testRegion)
	require.NoError(t, err)

	_, err = webPack.clt.Delete(ctx, deleteDraftEndpoint)
	require.NoError(t, err)

	_, err = client.GetDraftExternalAuditStorage(ctx)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	// Make sure an unauthenticated client can't delete draft cluster audit
	publicClt := s.client(t)
	_, err = publicClt.Delete(ctx, deleteDraftEndpoint)
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}
