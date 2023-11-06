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

	generateEndpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "integrations", "externalcloudaudit", "generate")
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
		"integrations",
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
