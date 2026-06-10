package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	v1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/services"
)

const validYaml = `kind: access_monitoring_rule
metadata:
  name: rule1
spec:
  subjects:
  - access_request
  states:
  - testing
  condition: "true"
  notification:
    recipients:
    - llama
    - alpaca
    name: slack
version: v1
`

func TestCreateAccessMonitoringRule(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	clusterName := s.testAuthServer.ClusterName()

	ruleMatchingValidYaml, err := services.NewAccessMonitoringRuleWithLabels("rule1", nil, pb.AccessMonitoringRuleSpec_builder{
		Subjects:  []string{types.KindAccessRequest},
		Condition: "true",
		States:    []string{"testing"},
		Notification: pb.Notification_builder{
			Name:       "slack",
			Recipients: []string{"llama", "alpaca"},
		}.Build(),
	}.Build())
	require.NoError(t, err)

	rule2, err := services.NewAccessMonitoringRuleWithLabels("rule2", nil, pb.AccessMonitoringRuleSpec_builder{
		Subjects:  []string{types.KindAccessRequest},
		Condition: "true",
		States:    []string{"testing2"},
		Notification: pb.Notification_builder{
			Name:       "slack",
			Recipients: []string{"apple", "banana"},
		}.Build(),
	}.Build())
	require.NoError(t, err)

	testcases := []struct {
		desc    string
		content string
		rule    *pb.AccessMonitoringRule
		expRule *pb.AccessMonitoringRule
		wantErr bool
	}{
		{
			desc:    "create yaml",
			content: validYaml,
			expRule: ruleMatchingValidYaml,
		},
		{
			desc:    "invalid yaml",
			content: "invalid yaml",
			wantErr: true,
		},
		{
			desc:    "create by object",
			rule:    rule2,
			expRule: rule2,
		},
	}

	endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "accessmonitoringrule")
	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			resp, err := webPack.clt.PostJSON(s.ctx, endpoint, ui.AccessMonitoringRuleWithYaml{
				YAML:   tc.content,
				Object: tc.rule,
			})

			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			var created ui.AccessMonitoringRuleWithYaml
			require.NoError(t, json.Unmarshal(resp.Bytes(), &created))
			require.Empty(t, cmp.Diff(created.Object, tc.expRule,
				protocmp.Transform(),
				protocmp.IgnoreFields(&v1.Metadata{}, "revision"),
			))
			require.NotEmpty(t, created.YAML)
		})
	}
}

func TestUpdateAccessMonitoringRule(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	clusterName := s.testAuthServer.ClusterName()
	authServer := s.testAuthServer.AuthServer.AuthServer

	ruleMatchingValidYaml, err := services.NewAccessMonitoringRuleWithLabels("rule1", nil, pb.AccessMonitoringRuleSpec_builder{
		Subjects:  []string{types.KindAccessRequest},
		Condition: "true",
		States:    []string{"testing"},
		Notification: pb.Notification_builder{
			Name:       "slack",
			Recipients: []string{"llama", "alpaca"},
		}.Build(),
	}.Build())
	require.NoError(t, err)

	createdRule, err := authServer.CreateAccessMonitoringRule(ctx, ruleMatchingValidYaml)
	require.NoError(t, err)

	// update yaml condition field
	const updatedYaml = `kind: access_monitoring_rule
metadata:
  name: rule1
  namespace: default
spec:
  subjects:
  - access_request
  states:
  - testing
  condition: "false"
  notification:
    recipients:
    - llama
    - alpaca
    name: slack
version: v1
`

	testcases := []struct {
		desc          string
		updateByYaml  string
		paramName     string
		wantErr       bool
		getUpdateRule func() *pb.AccessMonitoringRule
	}{
		{
			desc:         "update yaml",
			updateByYaml: updatedYaml,
			paramName:    "rule1",
			getUpdateRule: func() *pb.AccessMonitoringRule {
				createdRule.GetSpec().SetCondition("false")
				return createdRule
			},
		},
		{
			desc:         "invalid yaml",
			updateByYaml: "invalid yaml",
			wantErr:      true,
			paramName:    "rule1",
		},
		{
			desc:         "cannot rename",
			updateByYaml: updatedYaml,
			wantErr:      true,
			paramName:    "invalid-name",
		},
		{
			desc:      "udpate by resource object",
			paramName: "rule1",
			getUpdateRule: func() *pb.AccessMonitoringRule {
				createdRule.GetSpec().SetCondition("false")
				return createdRule
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			var expectedUpdatedRule *pb.AccessMonitoringRule
			if tc.getUpdateRule != nil {
				expectedUpdatedRule = tc.getUpdateRule()
			}

			endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "accessmonitoringrule", tc.paramName)
			resp, err := webPack.clt.PutJSON(s.ctx, endpoint, ui.AccessMonitoringRuleWithYaml{
				YAML:   tc.updateByYaml,
				Object: expectedUpdatedRule,
			})

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			var updated ui.AccessMonitoringRuleWithYaml
			require.NoError(t, json.Unmarshal(resp.Bytes(), &updated))
			require.Empty(t, cmp.Diff(updated.Object, expectedUpdatedRule,
				protocmp.Transform(),
				protocmp.IgnoreFields(&v1.Metadata{}, "revision"),
			))
			require.NotEmpty(t, updated.YAML)
		})
	}
}

func TestDeleteAccessMonitoringRule(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	clusterName := s.testAuthServer.ClusterName()
	authServer := s.testAuthServer.AuthServer.AuthServer

	rule, err := services.NewAccessMonitoringRuleWithLabels("rule1", nil, pb.AccessMonitoringRuleSpec_builder{
		Subjects:  []string{types.KindAccessRequest},
		Condition: "true",
		States:    []string{"testing"},
		Notification: pb.Notification_builder{
			Name:       "slack",
			Recipients: []string{"llama", "alpaca"},
		}.Build(),
	}.Build())
	require.NoError(t, err)

	createdRule, err := authServer.CreateAccessMonitoringRule(ctx, rule)
	require.NoError(t, err)

	// Test it was created in the backend.
	gotRule, err := authServer.GetAccessMonitoringRule(ctx, rule.GetMetadata().GetName())
	require.NoError(t, err)
	require.Equal(t, createdRule.GetMetadata().GetName(), gotRule.GetMetadata().GetName())

	endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "accessmonitoringrule", createdRule.GetMetadata().GetName())
	_, err = webPack.clt.Delete(s.ctx, endpoint)
	require.NoError(t, err)

	// Check it's been removed from backend.
	_, err = authServer.GetAccessMonitoringRule(ctx, rule.GetMetadata().GetName())
	require.True(t, trace.IsNotFound(err))
}

func TestGetAccessMonitoringRules_NoFilters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	clusterName := s.testAuthServer.ClusterName()
	authServer := s.testAuthServer.AuthServer.AuthServer

	rule1, err := services.NewAccessMonitoringRuleWithLabels("rule1", nil, pb.AccessMonitoringRuleSpec_builder{
		Subjects:  []string{types.KindAccessRequest},
		Condition: "true",
		Notification: pb.Notification_builder{
			Name: "slack",
		}.Build(),
	}.Build())
	require.NoError(t, err)
	_, err = authServer.CreateAccessMonitoringRule(ctx, rule1)
	require.NoError(t, err)

	rule2, err := services.NewAccessMonitoringRuleWithLabels("rule2", nil, pb.AccessMonitoringRuleSpec_builder{
		Subjects:  []string{"somethingElse"},
		Condition: "true",
		Notification: pb.Notification_builder{
			Name: "slack",
		}.Build(),
	}.Build())
	require.NoError(t, err)
	_, err = authServer.CreateAccessMonitoringRule(ctx, rule2)
	require.NoError(t, err)

	rule3, err := services.NewAccessMonitoringRuleWithLabels("rule3", nil, pb.AccessMonitoringRuleSpec_builder{
		Subjects:  []string{"somethingElse2"},
		Condition: "true",
		Notification: pb.Notification_builder{
			Name: "slack",
		}.Build(),
	}.Build())
	require.NoError(t, err)
	_, err = authServer.CreateAccessMonitoringRule(ctx, rule3)
	require.NoError(t, err)

	endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "accessmonitoringrule")
	resp, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
	require.NoError(t, err)

	var page ui.AccessMonitoringRulePage
	require.NoError(t, json.Unmarshal(resp.Bytes(), &page))
	require.Len(t, page.Rules, 3)
	require.Empty(t, page.StartKey)

	for _, rule := range page.Rules {
		require.NotEmpty(t, rule.Object)
		require.NotEmpty(t, rule.YAML)
	}
}

func TestGetAccessMonitoringRules_WithAccessRequestFilter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	clusterName := s.testAuthServer.ClusterName()
	authServer := s.testAuthServer.AuthServer.AuthServer

	randomSubjects := []string{
		"someSubject",
		"someSubject",
		"someSubject",
		"someSubject",
		"someSubject",
		"someSubject",
		"someSubject",
		"someSubject",
		types.KindAccessRequest,
		"someSubject",
		"someSubject",
		"someSubject",
		types.KindAccessRequest,
	}

	for i := range randomSubjects {
		rule, err := services.NewAccessMonitoringRuleWithLabels(fmt.Sprintf("rule%v", i), nil, pb.AccessMonitoringRuleSpec_builder{
			Subjects:  []string{randomSubjects[i]},
			Condition: "true",
			Notification: pb.Notification_builder{
				Name: "slack",
			}.Build(),
		}.Build())
		require.NoError(t, err)
		_, err = authServer.CreateAccessMonitoringRule(ctx, rule)
		require.NoError(t, err)
	}

	endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "accessmonitoringrule")
	resp, err := webPack.clt.Get(s.ctx, endpoint, url.Values{
		"limit":   []string{"3"},
		"subject": []string{types.KindAccessRequest},
	})
	require.NoError(t, err)

	var page ui.AccessMonitoringRulePage
	require.NoError(t, json.Unmarshal(resp.Bytes(), &page))
	require.Len(t, page.Rules, 2)

	for _, rule := range page.Rules {
		require.Equal(t, types.KindAccessRequest, rule.Object.GetSpec().GetSubjects()[0])
		require.NotEmpty(t, rule.YAML)
	}
}

const validAccessMonitoringRuleTerraform = `resource "teleport_access_monitoring_rule" "foo" {
  version = "v1"

  metadata = {
    name      = "foo"
    namespace = "default"
  }

  spec = {
    subjects  = ["access_request"]
    states    = ["testing"]
    condition = "true"
    notification = {
      name       = "mattermost"
      recipients = ["apple"]
    }
  }
}
`

func TestGetAccessMonitoringRuleTerraform_Valid(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	clusterName := s.testAuthServer.ClusterName()
	authServer := s.testAuthServer.AuthServer.AuthServer

	rule, err := services.NewAccessMonitoringRuleWithLabels("foo", nil, pb.AccessMonitoringRuleSpec_builder{
		Subjects:  []string{types.KindAccessRequest},
		Condition: "true",
		States:    []string{"testing"},
		Notification: pb.Notification_builder{
			Name:       "mattermost",
			Recipients: []string{"apple"},
		}.Build(),
	}.Build())
	require.NoError(t, err)
	_, err = authServer.CreateAccessMonitoringRule(s.ctx, rule)
	require.NoError(t, err)

	endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "accessmonitoringrule", rule.GetMetadata().GetName(), "terraform")
	re, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
	require.NoError(t, err)

	var resp accessMonitoringRuleGenerateTerraformResponse
	require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
	require.Equal(t, validAccessMonitoringRuleTerraform, resp.Terraform)
}
