/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/lib/services"
)

// This file is the future of the plugin integration testing.
// Previous integration tests relied on an existing teleport enterprise binary.
// This caused several issues, the main one being versionning plugins.
// Once all plugins are migrated to this new test suite, we'll be able to
// remove most of the integration package.

const (
	// RulerUserName is the name of the admin user.
	// Its client has full admin access to Teleport and can be used to setup
	// fixtures or approve requests in OSS tests
	RulerUserName = "admin"
	// RequesterOSSUserName is the user allowed to request RequestedRoleName.
	// Their role does not have any approval threshold and is compatible with
	// Teleport OSS.
	RequesterOSSUserName = "requester-oss@example.com"
	// Requester1UserName is the name of main role requester. They are allowed
	// to request RequestedRoleName, but require two approvals. This user is
	// only created in Enterprise tests.
	Requester1UserName = "requester1@example.com"
	// Requester2UserName is the name of secondary role requester.
	// Like Requester1UserName they need 2 approvals. This user is used in some
	// auto-approval race tests to see how the plugin behaves when several users
	// request the same role. This user is only created in Enterprise tests.
	Requester2UserName = "requester2@example.com"
	// Reviewer1UserName is one of the two access reviewers. This user is used
	// to test advanced approval workflows (multiple approvals required).
	// This user is only created in enterprise tests.
	Reviewer1UserName = "reviewer1@example.com"
	// Reviewer2UserName is exactly like Reviewer1UserName.
	// This user is only created in enterprise tests.
	Reviewer2UserName = "reviewer2@example.com"
	// PluginUserName is the Teleport user for the plugin.
	PluginUserName = "plugin"

	RequestedRoleName         = teleport.PresetEditorRoleName
	OSSRequesterRoleName      = "oss-requester"
	AdvancedRequesterRoleName = "advanced-requester"
	ReviewerRoleName          = "reviewer"
)

// AccessRequestSuite is the base test suite for access requests plugins.
// It sets up a single Teleport server for all the tests and creates the following fixtures:
// - Ruler user (admin client)
// - Requester1 and Requester2 users with the Requester role and an approval threshold of
// - reviewer users 1 and 2 with the reviewer role (only when running against teleport.e)
// - access plugin user and roles (a role for access requests, and another for access lists)
//
// It also signs an identity for the plugin and generates a working teleport
// client configuration.
type AccessRequestSuite struct {
	suite.Suite
	AuthHelper       AuthHelper
	clients          map[string]*Client
	teleportConfig   lib.TeleportConfig
	teleportFeatures *proto.Features

	// requestPadding allows tests to pad the access requests reason to test
	// how the plugin behaves when a message exceeds the max size.
	requestPadding int
}

// SetupSuite runs once for the whole test suite.
// It starts a Teleport instance, creates all the fixtures (users and roles).
func (s *AccessRequestSuite) SetupSuite() {
	var err error
	t := s.T()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	s.clients = make(map[string]*Client)

	// Start the Teleport Auth server and get the admin client.
	adminClient := s.AuthHelper.StartServer(t)
	s.clients[RulerUserName] = NewAccessRequestClient(adminClient)

	// Check the client and recover the cluster features
	pong, err := adminClient.Ping(ctx)
	require.NoError(t, err)
	s.teleportFeatures = pong.ServerFeatures

	// Create the editor role. This is is the role that will be requested.
	requestedRole := services.NewPresetEditorRole()
	requestedRole, err = adminClient.CreateRole(ctx, requestedRole)
	require.NoError(t, err)

	// Create the OSS Requester role and user.
	// This is a simple role allowing to request access, no threshold, advanced workflow.
	// This is compatible with Teleport OSS.
	ossRequesterRole, err := types.NewRole(OSSRequesterRoleName, types.RoleSpecV6{Allow: types.RoleConditions{
		Request: &types.AccessRequestConditions{
			Roles: []string{requestedRole.GetName()},
		}}})
	require.NoError(t, err)
	ossRequesterRole, err = adminClient.CreateRole(ctx, ossRequesterRole)
	require.NoError(t, err)

	ossRequester, err := types.NewUser(RequesterOSSUserName)
	require.NoError(t, err)
	ossRequester.SetRoles([]string{ossRequesterRole.GetName()})
	ossRequester, err = adminClient.CreateUser(ctx, ossRequester)
	require.NoError(t, err)

	s.clients[RequesterOSSUserName] = NewAccessRequestClient(s.newClientForUser(ctx, ossRequester))

	// If AdvancedAccessWorkflows is enabled (Teleport Enterprise) we will test
	// review thresholds and multiple reviewers.
	// Else we'll just use the Ruler user to approve/deny requests.
	if s.teleportFeatures.AdvancedAccessWorkflows {
		// Create the role and users for advanced access request workflows
		advancedRequesterRole, err := types.NewRole(AdvancedRequesterRoleName, types.RoleSpecV6{Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Roles:      []string{requestedRole.GetName()},
				Thresholds: []types.AccessReviewThreshold{{Approve: 2, Deny: 2}},
			}}})
		require.NoError(t, err)
		advancedRequesterRole, err = adminClient.CreateRole(ctx, advancedRequesterRole)
		require.NoError(t, err)
		requester1, err := types.NewUser(Requester1UserName)
		require.NoError(t, err)
		requester1.SetRoles([]string{advancedRequesterRole.GetName()})
		requester1, err = adminClient.CreateUser(ctx, requester1)
		require.NoError(t, err)

		requester2, err := types.NewUser(Requester2UserName)
		require.NoError(t, err)
		requester2.SetRoles([]string{advancedRequesterRole.GetName()})
		requester2, err = adminClient.CreateUser(ctx, requester2)
		require.NoError(t, err)

		// Create the reviewer role and the two reviewer users
		reviewerRole, err := types.NewRole(ReviewerRoleName, types.RoleSpecV6{
			Allow: types.RoleConditions{
				ReviewRequests: &types.AccessReviewConditions{
					Roles: []string{requestedRole.GetName()},
				},
			}})
		require.NoError(t, err)
		reviewerRole, err = adminClient.CreateRole(ctx, reviewerRole)
		require.NoError(t, err)

		reviewer1, err := types.NewUser(Reviewer1UserName)
		require.NoError(t, err)
		reviewer1.SetRoles([]string{reviewerRole.GetName()})
		reviewer1, err = adminClient.CreateUser(ctx, reviewer1)
		require.NoError(t, err)

		reviewer2, err := types.NewUser(Reviewer2UserName)
		require.NoError(t, err)
		reviewer2.SetRoles([]string{reviewerRole.GetName()})
		reviewer2, err = adminClient.CreateUser(ctx, reviewer2)
		require.NoError(t, err)

		// Build and store the teleport clients
		s.clients[Requester1UserName] = NewAccessRequestClient(s.newClientForUser(ctx, requester1))
		s.clients[Requester2UserName] = NewAccessRequestClient(s.newClientForUser(ctx, requester2))
		s.clients[Reviewer1UserName] = NewAccessRequestClient(s.newClientForUser(ctx, reviewer1))
		s.clients[Reviewer2UserName] = NewAccessRequestClient(s.newClientForUser(ctx, reviewer2))
	}

	// Create the role for the plugin to watch access requests and write plugin data
	pluginRole, err := types.NewRole("access-plugin", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				// The verb "update" is not required unless the plugin has the ability to approve access requests
				// The ability to approve an access request is different from the ability to review an access request.
				// The Jira plugin approves access requests while the pagerduty/snow/opsgenie plugins
				// submit positive reviews.
				types.NewRule("access_request", []string{"list", "read", "update"}),
				types.NewRule("access_plugin_data", []string{"update"}),
				types.NewRule(types.KindAccessMonitoringRule, []string{"update", "read", "list"}),
			},
		},
	})
	require.NoError(t, err)
	pluginRole, err = adminClient.CreateRole(ctx, pluginRole)
	require.NoError(t, err)

	// Create the role for the reminder plugin to read access lists
	reminderRole, err := types.NewRole("reminder-plugin", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule("access_list", []string{"list", "read"}),
			},
		},
	})
	require.NoError(t, err)
	reminderRole, err = adminClient.CreateRole(ctx, reminderRole)
	require.NoError(t, err)

	pluginRoles := []string{pluginRole.GetName(), reminderRole.GetName()}

	// Auto approval requires setting some fields that are enterprise-only.
	// We must skip role creation in OSS, else it will fail.
	if s.TeleportFeatures().AdvancedAccessWorkflows {
		// Create the role for the plugin to automatically approve access requests
		autoApprovalRole, err := types.NewRole("auto-approval-plugin", types.RoleSpecV6{
			Allow: types.RoleConditions{
				ReviewRequests: &types.AccessReviewConditions{
					Roles: []string{RequestedRoleName},
				},
			},
		})
		require.NoError(t, err)
		autoApprovalRole, err = adminClient.CreateRole(ctx, autoApprovalRole)
		require.NoError(t, err)

		pluginRoles = append(pluginRoles, autoApprovalRole.GetName())
	}

	pluginUser, err := types.NewUser(PluginUserName)
	require.NoError(t, err)
	pluginUser.SetRoles(pluginRoles)
	pluginUser, err = adminClient.CreateUser(ctx, pluginUser)
	require.NoError(t, err)

	// Sign an identity for the access plugin and generate its configuration
	s.teleportConfig.Addr = s.AuthHelper.ServerAddr()
	s.teleportConfig.Identity = s.AuthHelper.SignIdentityForUser(t, ctx, pluginUser)
}

// newClientForUser creates a teleport client for a give user.
// The user must be created beforehand.
func (s *AccessRequestSuite) newClientForUser(ctx context.Context, user types.User) *client.Client {
	s.T().Helper()
	t := s.T()
	creds := s.AuthHelper.CredentialsForUser(t, ctx, user)
	clientConfig := client.Config{
		Addrs:       []string{s.AuthHelper.ServerAddr()},
		Credentials: []client.Credentials{creds},
	}
	userClient, err := client.New(ctx, clientConfig)
	require.NoError(t, err)
	_, err = userClient.Ping(ctx)
	require.NoError(t, err)
	return userClient
}

// Ruler returns the AccessRequestClient for the Ruler user
func (s *AccessRequestSuite) Ruler() *Client {
	return s.clients[RulerUserName]
}

// RequesterOSS returns the AccessRequestClient for the RequesterOSS user
func (s *AccessRequestSuite) RequesterOSS() *Client {
	return s.clients[RequesterOSSUserName]
}

// Requester1 returns the AccessRequestClient for the Requester1 user
func (s *AccessRequestSuite) Requester1() *Client {
	return s.clients[Requester1UserName]
}

// Requester2 returns the AccessRequestClient for the Requester2 user
func (s *AccessRequestSuite) Requester2() *Client {
	return s.clients[Requester2UserName]
}

// Reviewer1 returns the AccessRequestClient for the Reviewer1 user
func (s *AccessRequestSuite) Reviewer1() *Client {
	return s.clients[Reviewer1UserName]
}

// Reviewer2 returns the AccessRequestClient for the Reviewer2 user
func (s *AccessRequestSuite) Reviewer2() *Client {
	return s.clients[Reviewer2UserName]
}

// ClientByName returns the AccessRequestClient for any user.
// While this can be done via the nice helper functions like Ruler(),
// there are cases where we want to get clients based on a username
// (see race tests where the username is in a variable)
func (s *AccessRequestSuite) ClientByName(name string) *Client {
	return s.clients[name]
}

// NewAccessRequest creates an access request.
// The access request reason can be padded with "A" by setting
// SetReasonPadding.
func (s *AccessRequestSuite) NewAccessRequest(userName string, suggestedReviewers []string, padding int) types.AccessRequest {
	s.T().Helper()
	t := s.T()
	t.Helper()

	reason := "because of"
	if padding > 0 {
		reason = reason + " " + strings.Repeat("A", padding)
	}

	req, err := types.NewAccessRequest(uuid.New().String(), userName, RequestedRoleName)
	require.NoError(t, err)
	req.SetRequestReason(reason)
	req.SetSuggestedReviewers(suggestedReviewers)

	return req
}

// CreateAccessRequest creates a new access request and submits it.
func (s *AccessRequestSuite) CreateAccessRequest(ctx context.Context, userName string, suggestedReviewers []string) types.AccessRequest {
	s.T().Helper()
	t := s.T()

	req := s.NewAccessRequest(userName, suggestedReviewers, s.requestPadding)
	out, err := s.ClientByName(userName).CreateAccessRequestV2(ctx, req)

	require.NoError(t, err)
	return out
}

// TeleportFeatures returns the teleport features of the auth server the tests
// are running against.
func (s *AccessRequestSuite) TeleportFeatures() *proto.Features {
	return s.teleportFeatures
}

// TeleportConfig returns a valid teleport config for the auth server the tests
// are running against. This config can then be passed to plugins.
func (s *AccessRequestSuite) TeleportConfig() lib.TeleportConfig {
	return s.teleportConfig
}

// SetReasonPadding sets the padding when creating access request. This is used
// to test how plugins are behaving when too large messages are sent.
func (s *AccessRequestSuite) SetReasonPadding(padding int) {
	s.requestPadding = padding
}

// AnnotateRequesterRoleAccessRequests sets the access request annotations on both
// requester roles (OSS and Advanced workflows). Those annotations can then be
// used to route notifications to specific channels, or trigger automatic approval.
func (s *AccessRequestSuite) AnnotateRequesterRoleAccessRequests(ctx context.Context, annotationKey string, annotationValue []string) {
	t := s.T()
	t.Helper()
	adminClient := s.Ruler()

	// If we're running in OSS, we have a single requester role, but if we're
	// running against an enterprise server we also have the advanced requester.
	roles := []string{OSSRequesterRoleName}
	if s.TeleportFeatures().AdvancedAccessWorkflows {
		roles = append(roles, AdvancedRequesterRoleName)
	}
	for _, roleName := range roles {
		role, err := adminClient.GetRole(ctx, roleName)
		require.NoError(t, err)
		conditions := role.GetAccessRequestConditions(types.Allow)
		if conditions.Annotations == nil {
			conditions.Annotations = make(map[string][]string)
		}
		conditions.Annotations[annotationKey] = annotationValue
		role.SetAccessRequestConditions(types.Allow, conditions)
		_, err = adminClient.UpdateRole(ctx, role)
		require.NoError(t, err)
	}
}

func (s *AccessRequestSuite) RequireAdvancedWorkflow(t *testing.T) {
	if !s.TeleportFeatures().GetAdvancedAccessWorkflows() {
		require.Fail(t, "This test requires AdvancedAccessWorkflows (Teleport enterprise)")
	}
}
