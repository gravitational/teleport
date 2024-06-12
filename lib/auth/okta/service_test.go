/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package okta

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestOktaImportRules(t *testing.T) {
	ctx, svc := initSvc(t, types.KindOktaImportRule)

	listResp, err := svc.ListOktaImportRules(ctx, &oktapb.ListOktaImportRulesRequest{})
	require.NoError(t, err)
	require.Empty(t, listResp.NextPageToken)
	require.Empty(t, listResp.ImportRules)

	r1 := newOktaImportRule(t, "1")
	r2 := newOktaImportRule(t, "2")
	r3 := newOktaImportRule(t, "3")

	createResp, err := svc.CreateOktaImportRule(ctx, &oktapb.CreateOktaImportRuleRequest{ImportRule: r1})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(r1, createResp))

	createResp, err = svc.CreateOktaImportRule(ctx, &oktapb.CreateOktaImportRuleRequest{ImportRule: r2})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(r2, createResp))

	createResp, err = svc.CreateOktaImportRule(ctx, &oktapb.CreateOktaImportRuleRequest{ImportRule: r3})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(r3, createResp))

	listResp, err = svc.ListOktaImportRules(ctx, &oktapb.ListOktaImportRulesRequest{})
	require.NoError(t, err)
	require.Empty(t, listResp.NextPageToken)
	require.Empty(t, cmp.Diff([]*types.OktaImportRuleV1{r1, r2, r3}, listResp.ImportRules,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	r1.SetExpiry(time.Now().Add(30 * time.Minute))
	updateResp, err := svc.UpdateOktaImportRule(ctx, &oktapb.UpdateOktaImportRuleRequest{ImportRule: r1})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(r1, updateResp))

	r, err := svc.GetOktaImportRule(ctx, &oktapb.GetOktaImportRuleRequest{Name: r1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(r1, r,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	_, err = svc.DeleteOktaImportRule(ctx, &oktapb.DeleteOktaImportRuleRequest{Name: r1.GetName()})
	require.NoError(t, err)

	listResp, err = svc.ListOktaImportRules(ctx, &oktapb.ListOktaImportRulesRequest{})
	require.NoError(t, err)
	require.Empty(t, listResp.NextPageToken)
	require.Empty(t, cmp.Diff([]*types.OktaImportRuleV1{r2, r3}, listResp.ImportRules,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	_, err = svc.DeleteAllOktaImportRules(ctx, &oktapb.DeleteAllOktaImportRulesRequest{})
	require.NoError(t, err)

	listResp, err = svc.ListOktaImportRules(ctx, &oktapb.ListOktaImportRulesRequest{})
	require.NoError(t, err)
	require.Empty(t, listResp.NextPageToken)
	require.Empty(t, listResp.ImportRules)
}

func TestOktaAssignments(t *testing.T) {
	ctx, svc := initSvc(t, types.KindOktaAssignment)

	listResp, err := svc.ListOktaAssignments(ctx, &oktapb.ListOktaAssignmentsRequest{})
	require.NoError(t, err)
	require.Empty(t, listResp.NextPageToken)
	require.Empty(t, listResp.Assignments)

	a1 := newOktaAssignment(t, "1")
	a2 := newOktaAssignment(t, "2")
	a3 := newOktaAssignment(t, "3")

	createResp, err := svc.CreateOktaAssignment(ctx, &oktapb.CreateOktaAssignmentRequest{Assignment: a1})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1, createResp))

	createResp, err = svc.CreateOktaAssignment(ctx, &oktapb.CreateOktaAssignmentRequest{Assignment: a2})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a2, createResp))

	createResp, err = svc.CreateOktaAssignment(ctx, &oktapb.CreateOktaAssignmentRequest{Assignment: a3})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a3, createResp))

	listResp, err = svc.ListOktaAssignments(ctx, &oktapb.ListOktaAssignmentsRequest{})
	require.NoError(t, err)
	require.Empty(t, listResp.NextPageToken)
	require.Empty(t, cmp.Diff([]*types.OktaAssignmentV1{a1, a2, a3}, listResp.Assignments,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	a1.SetExpiry(time.Now().Add(30 * time.Minute))
	updateResp, err := svc.UpdateOktaAssignment(ctx, &oktapb.UpdateOktaAssignmentRequest{Assignment: a1})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1, updateResp))

	a, err := svc.GetOktaAssignment(ctx, &oktapb.GetOktaAssignmentRequest{Name: a1.GetName()})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(a1, a,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	_, err = svc.UpdateOktaAssignmentStatus(ctx, &oktapb.UpdateOktaAssignmentStatusRequest{
		Name:   a1.GetName(),
		Status: types.OktaAssignmentSpecV1_PROCESSING,
	})
	require.NoError(t, err)

	require.NoError(t, a1.SetStatus(constants.OktaAssignmentStatusProcessing))
	a, err = svc.GetOktaAssignment(ctx, &oktapb.GetOktaAssignmentRequest{Name: a1.GetName()})
	require.NoError(t, err)
	a1.SetLastTransition(a.GetLastTransition())
	require.Empty(t, cmp.Diff(a1, a,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	_, err = svc.DeleteOktaAssignment(ctx, &oktapb.DeleteOktaAssignmentRequest{Name: a1.GetName()})
	require.NoError(t, err)

	listResp, err = svc.ListOktaAssignments(ctx, &oktapb.ListOktaAssignmentsRequest{})
	require.NoError(t, err)
	require.Empty(t, listResp.NextPageToken)
	require.Empty(t, cmp.Diff([]*types.OktaAssignmentV1{a2, a3}, listResp.Assignments,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	_, err = svc.DeleteAllOktaAssignments(ctx, &oktapb.DeleteAllOktaAssignmentsRequest{})
	require.NoError(t, err)

	listResp, err = svc.ListOktaAssignments(ctx, &oktapb.ListOktaAssignmentsRequest{})
	require.NoError(t, err)
	require.Empty(t, listResp.NextPageToken)
	require.Empty(t, listResp.Assignments)
}

type testClient struct {
	services.ClusterConfiguration
	services.Trust
	services.RoleGetter
	services.UserGetter
}

func initSvc(t *testing.T, kind string) (context.Context, *Service) {
	ctx := context.Background()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	clusterConfigSvc, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
	trustSvc := local.NewCAService(backend)
	roleSvc := local.NewAccessService(backend)
	userSvc := local.NewTestIdentityService(backend)

	_, err = clusterConfigSvc.UpsertAuthPreference(ctx, types.DefaultAuthPreference())
	require.NoError(t, err)
	require.NoError(t, clusterConfigSvc.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig()))
	_, err = clusterConfigSvc.UpsertClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
	require.NoError(t, err)
	_, err = clusterConfigSvc.UpsertSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())
	require.NoError(t, err)

	accessPoint := &testClient{
		ClusterConfiguration: clusterConfigSvc,
		Trust:                trustSvc,
		RoleGetter:           roleSvc,
		UserGetter:           userSvc,
	}

	accessService := local.NewAccessService(backend)
	eventService := local.NewEventsService(backend)
	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Client:    eventService,
			Component: "test",
		},
		LockGetter: accessService,
	})
	require.NoError(t, err)

	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: "test-cluster",
		AccessPoint: accessPoint,
		LockWatcher: lockWatcher,
	})
	require.NoError(t, err)

	role, err := types.NewRole("import-rules", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{kind},
					Verbs:     []string{types.VerbList, types.VerbRead, types.VerbUpdate, types.VerbCreate, types.VerbDelete},
				},
			},
		},
	})
	require.NoError(t, err)
	role, err = roleSvc.CreateRole(ctx, role)
	require.NoError(t, err)

	user, err := types.NewUser("test-user")
	user.AddRole(role.GetName())
	require.NoError(t, err)
	user, err = userSvc.CreateUser(ctx, user)
	require.NoError(t, err)

	svc, err := NewService(ServiceConfig{
		Backend:    backend,
		Authorizer: authorizer,
	})
	require.NoError(t, err)

	ctx = authz.ContextWithUser(ctx, authz.LocalUser{
		Username: user.GetName(),
		Identity: tlsca.Identity{
			Username: user.GetName(),
			Groups:   []string{role.GetName()},
		},
	})

	return ctx, svc
}

func newOktaImportRule(t *testing.T, name string) *types.OktaImportRuleV1 {
	importRule, err := types.NewOktaImportRule(
		types.Metadata{
			Name: name,
		},
		types.OktaImportRuleSpecV1{
			Mappings: []*types.OktaImportRuleMappingV1{
				{
					Match: []*types.OktaImportRuleMatchV1{
						{
							AppIDs: []string{"yes"},
						},
					},
					AddLabels: map[string]string{
						"label1": "value1",
					},
				},
				{
					Match: []*types.OktaImportRuleMatchV1{
						{
							GroupIDs: []string{"yes"},
						},
					},
					AddLabels: map[string]string{
						"label1": "value1",
					},
				},
			},
		},
	)
	require.NoError(t, err)

	return importRule.(*types.OktaImportRuleV1)
}

func newOktaAssignment(t *testing.T, name string) *types.OktaAssignmentV1 {
	assignment, err := types.NewOktaAssignment(
		types.Metadata{
			Name: name,
		},
		types.OktaAssignmentSpecV1{
			User: "test-user@test.user",
			Targets: []*types.OktaAssignmentTargetV1{
				{
					Type: types.OktaAssignmentTargetV1_APPLICATION,
					Id:   "123456",
				},
				{
					Type: types.OktaAssignmentTargetV1_GROUP,
					Id:   "234567",
				},
			},
			Status: types.OktaAssignmentSpecV1_PENDING,
		},
	)
	require.NoError(t, err)

	return assignment.(*types.OktaAssignmentV1)
}
