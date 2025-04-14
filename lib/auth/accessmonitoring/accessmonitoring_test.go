/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

package accessmonitoring

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

const (
	adminRoleName     = "admin-role"
	requesterRoleName = "requester-role"
	dynamicRoleName   = "dynamic-role"

	// admin-user is granted permissions to create access monitoring rules.
	adminUserName = "admin-user"
	// requester-user is granted permissions to create access requests for the
	// dynamic-role.
	requesterUserName = "requester-user"
)

func TestAccessMonitoringSuite(t *testing.T) {
	suite.Run(t, &AccessMonitoringSuite{})
}

type AccessMonitoringSuite struct {
	suite.Suite
	srv *auth.TestTLSServer
}

func (s *AccessMonitoringSuite) SetupTest() {
	t := s.T()
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	s.srv = newTestTLSServer(t)

	// Setup system access review bot role and user.
	_, err := s.srv.Auth().UpsertRole(ctx, services.NewSystemAutomaticAccessApproverRole())
	require.NoError(t, err)

	_, err = s.srv.Auth().UpsertUser(ctx, services.NewSystemAutomaticAccessBotUser())
	require.NoError(t, err)

	// Setup admin role and user
	adminRole, err := types.NewRole(adminRoleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindAccessMonitoringRule, services.RW()),
			},
		},
	})
	require.NoError(t, err)

	_, err = s.srv.Auth().UpsertRole(ctx, adminRole)
	require.NoError(t, err)

	adminUser, err := types.NewUser(adminUserName)
	require.NoError(t, err)

	adminUser.SetRoles([]string{adminRoleName})
	_, err = s.srv.Auth().UpsertUser(ctx, adminUser)
	require.NoError(t, err)

	// Setup requester role and user
	requesterRole, err := types.NewRole(requesterRoleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Roles: []string{dynamicRoleName},
			},
			Rules: []types.Rule{
				types.NewRule(types.KindAccessRequest, services.RW()),
			},
		},
	})
	require.NoError(t, err)

	_, err = s.srv.Auth().UpsertRole(ctx, requesterRole)
	require.NoError(t, err)

	requesterUser, err := types.NewUser(requesterUserName)
	require.NoError(t, err)

	requesterUser.SetRoles([]string{requesterRoleName})
	_, err = s.srv.Auth().UpsertUser(ctx, requesterUser)
	require.NoError(t, err)

	// Setup dynamic role
	dynamicRole, err := types.NewRole(dynamicRoleName, types.RoleSpecV6{})
	require.NoError(t, err)

	_, err = s.srv.Auth().UpsertRole(ctx, dynamicRole)
	require.NoError(t, err)
}

func (s *AccessMonitoringSuite) TestAccessRequestApproved() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	// Initialize and run access monitoring service
	accessMonitoringService, err := NewAccessMonitoringService(Config{
		Backend: s.srv.AuthServer.Backend,
		Client:  s.srv.Auth(),
	})
	require.NoError(t, err)
	go func() { require.NoError(t, accessMonitoringService.Run(ctx)) }()

	// Setup access monitoring rules
	adminClient, err := s.srv.NewClient(auth.TestUser(adminUserName))
	require.NoError(t, err)

	rule := newApprovedRule("approve-dynamic-role", `
		contains_all(set("dynamic-role"), access_request.spec.roles)`)

	_, err = adminClient.AccessMonitoringRuleClient().CreateAccessMonitoringRule(ctx, rule)
	require.NoError(t, err)

	// Create access request
	requesterClient, err := s.srv.NewClient(auth.TestUser(requesterUserName))
	require.NoError(t, err)

	req, err := services.NewAccessRequest(requesterUserName, dynamicRoleName)
	require.NoError(t, err)

	rr, err := requesterClient.CreateAccessRequestV2(ctx, req)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		resp, err := s.srv.Auth().GetAccessRequests(ctx, types.AccessRequestFilter{
			ID: rr.GetName(),
		})
		require.NoError(t, err)
		require.Len(t, resp, 1)
		require.Equal(t, types.RequestState_APPROVED, resp[0].GetState())
	}, 10*time.Second, 100*time.Millisecond)
}

func (s *AccessMonitoringSuite) TestAccessRequestDenied() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	// Initialize and run access monitoring service
	accessMonitoringService, err := NewAccessMonitoringService(Config{
		Backend: s.srv.AuthServer.Backend,
		Client:  s.srv.Auth(),
	})
	require.NoError(t, err)
	go func() { require.NoError(t, accessMonitoringService.Run(ctx)) }()

	// Setup access monitoring rules
	adminClient, err := s.srv.NewClient(auth.TestUser(adminUserName))
	require.NoError(t, err)

	rule := newDeniedRule("deny-dynamic-role", `
		contains_all(set("dynamic-role"), access_request.spec.roles)`)

	_, err = adminClient.AccessMonitoringRuleClient().CreateAccessMonitoringRule(ctx, rule)
	require.NoError(t, err)

	// Create access request
	requesterClient, err := s.srv.NewClient(auth.TestUser(requesterUserName))
	require.NoError(t, err)

	req, err := services.NewAccessRequest(requesterUserName, dynamicRoleName)
	require.NoError(t, err)

	rr, err := requesterClient.CreateAccessRequestV2(ctx, req)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		resp, err := s.srv.Auth().GetAccessRequests(ctx, types.AccessRequestFilter{
			ID: rr.GetName(),
		})
		require.NoError(t, err)
		require.Len(t, resp, 1)
		require.Equal(t, types.RequestState_DENIED, resp[0].GetState())
	}, 10*time.Second, 100*time.Millisecond)
}

func newTestTLSServer(t testing.TB) *auth.TestTLSServer {
	as, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err)

	srv, err := as.NewTestTLSServer()
	require.NoError(t, err)

	t.Cleanup(func() {
		err := srv.Close()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		require.NoError(t, err)
	})

	return srv
}

func newApprovedRule(name, condition string) *accessmonitoringrulesv1.AccessMonitoringRule {
	return newReviewRule(name, condition, types.RequestState_APPROVED.String())
}

func newDeniedRule(name, condition string) *accessmonitoringrulesv1.AccessMonitoringRule {
	return newReviewRule(name, condition, types.RequestState_DENIED.String())
}

func newReviewRule(name, condition, decision string) *accessmonitoringrulesv1.AccessMonitoringRule {
	return &accessmonitoringrulesv1.AccessMonitoringRule{
		Kind:    types.KindAccessMonitoringRule,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
			Subjects:     []string{types.KindAccessRequest},
			Condition:    condition,
			DesiredState: types.AccessMonitoringRuleStateReviewed,
			AutomaticReview: &accessmonitoringrulesv1.AutomaticReview{
				Integration: types.BuiltInAutomaticReview,
				Decision:    decision,
			},
		},
	}
}
