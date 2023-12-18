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

package opsgenie

import (
	"os/user"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
)

const (
	NotifyServiceName       = "Teleport Notifications"
	NotifyServiceAnnotation = types.TeleportNamespace + types.ReqAnnotationNotifyServicesLabel
	ResponderName1          = "Responder 1"
	ResponderName2          = "Responder 2"
	ResponderName3          = "Responder 3"
)

type OpsgenieSuite struct {
	integration.Suite
	appConfig        Config
	currentRequestor string
	userNames        struct {
		ruler     string
		reviewer1 string
		reviewer2 string
		requestor string
		approver  string
		racer1    string
		racer2    string
		plugin    string
	}
	raceNumber   int
	fakeOpsgenie *FakeOpsgenie

	ogNotifyResponder Responder
	ogResponder1      Responder
	ogResponder2      Responder
	ogResponder3      Responder

	clients          map[string]*integration.Client
	teleportFeatures *proto.Features
	teleportConfig   lib.TeleportConfig
}

func TestOpsgenieSuite(t *testing.T) { suite.Run(t, &OpsgenieSuite{}) }

func (s *OpsgenieSuite) SetupSuite() {
	var err error
	t := s.T()

	logger.Init()
	err = logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)
	s.raceNumber = 2 * runtime.GOMAXPROCS(0)
	me, err := user.Current()
	require.NoError(t, err)

	// We set such a big timeout because integration.NewFromEnv could start
	// downloading a Teleport *-bin.tar.gz file which can take a long time.
	ctx := s.SetContextTimeout(2 * time.Minute)

	teleport, err := integration.NewFromEnv(ctx)
	require.NoError(t, err)
	require.NotNil(t, teleport)
	t.Cleanup(teleport.Close)

	auth, err := teleport.NewAuthService()
	require.NoError(t, err)
	require.NotNil(t, auth)
	s.StartApp(auth)

	s.clients = make(map[string]*integration.Client)

	// Set up the user who has an access to all kinds of resources.

	s.userNames.ruler = me.Username + "-ruler@example.com"
	client, err := teleport.MakeAdmin(ctx, auth, s.userNames.ruler)
	require.NoError(t, err)
	s.clients[s.userNames.ruler] = client

	// Get the server features.

	pong, err := client.Ping(ctx)
	require.NoError(t, err)
	teleportFeatures := pong.GetServerFeatures()

	var bootstrap integration.Bootstrap

	// Set up user who can request the access to role "editor".

	conditions := types.RoleConditions{
		Request: &types.AccessRequestConditions{
			Roles: []string{"editor"},
			Annotations: wrappers.Traits{
				NotifyServiceAnnotation: []string{NotifyServiceName},
			},
		},
	}
	if teleportFeatures.AdvancedAccessWorkflows {
		conditions.Request.Thresholds = []types.AccessReviewThreshold{{Approve: 2, Deny: 2}}
	}
	// This is the role for testing notification alert creation.
	role, err := bootstrap.AddRole("foo", types.RoleSpecV6{Allow: conditions})
	require.NoError(t, err)

	user, err := bootstrap.AddUserWithRoles(me.Username+"@example.com", role.GetName())
	require.NoError(t, err)
	s.userNames.requestor = user.GetName()

	if teleportFeatures.AdvancedAccessWorkflows {
		// Set up TWO users who can review access requests to role "editor".

		role, err = bootstrap.AddRole("foo-reviewer", types.RoleSpecV6{
			Allow: types.RoleConditions{
				ReviewRequests: &types.AccessReviewConditions{Roles: []string{"editor"}},
			},
		})
		require.NoError(t, err)

		user, err = bootstrap.AddUserWithRoles(me.Username+"-reviewer1@example.com", role.GetName())
		require.NoError(t, err)
		s.userNames.reviewer1 = user.GetName()

		user, err = bootstrap.AddUserWithRoles(me.Username+"-reviewer2@example.com", role.GetName())
		require.NoError(t, err)
		s.userNames.reviewer2 = user.GetName()

		// This is the role that needs exactly one approval review for an access request to be approved.
		// It's handy to test auto-approval scenarios so we also put "opsgenie_services" annotation.
		role, err = bootstrap.AddRole("bar", types.RoleSpecV6{
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles: []string{"editor"},
					Annotations: wrappers.Traits{
						NotifyServiceAnnotation: []string{ResponderName1, ResponderName2},
					},
				},
			},
		})
		require.NoError(t, err)

		user, err = bootstrap.AddUserWithRoles(me.Username+"-approver@example.com", role.GetName())
		require.NoError(t, err)
		s.userNames.approver = user.GetName()

		// This is the role with a maximum possible setup: both "opsgenie_notify_service" and
		// "opsgenie_services" annotations and threshold.
		role, err = bootstrap.AddRole("foo-bar", types.RoleSpecV6{
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles: []string{"editor"},
					Annotations: wrappers.Traits{
						NotifyServiceAnnotation: []string{NotifyServiceName},
						// ServicesDefaultAnnotation: []string{ServiceName1, ServiceName2}, // TODO: FIX THIS
					},
					Thresholds: []types.AccessReviewThreshold{types.AccessReviewThreshold{Approve: 2, Deny: 2}},
				},
			},
		})
		require.NoError(t, err)

		user, err = bootstrap.AddUserWithRoles(me.Username+"-racer1@example.com", role.GetName())
		require.NoError(t, err)
		s.userNames.racer1 = user.GetName()

		user, err = bootstrap.AddUserWithRoles(me.Username+"-racer2@example.com", role.GetName())
		require.NoError(t, err)
		s.userNames.racer2 = user.GetName()
	}

	conditions = types.RoleConditions{
		Rules: []types.Rule{
			types.NewRule("access_request", []string{"list", "read"}),
			types.NewRule("access_plugin_data", []string{"update"}),
		},
	}
	if teleportFeatures.AdvancedAccessWorkflows {
		conditions.ReviewRequests = &types.AccessReviewConditions{Roles: []string{"editor"}}
	}

	// Set up plugin user.

	role, err = bootstrap.AddRole("access-opsgenie", types.RoleSpecV6{Allow: conditions})
	require.NoError(t, err)

	user, err = bootstrap.AddUserWithRoles("access-opsgenie", role.GetName())
	require.NoError(t, err)
	s.userNames.plugin = user.GetName()

	// Bake all the resources.

	err = teleport.Bootstrap(ctx, auth, bootstrap.Resources())
	require.NoError(t, err)

	// Initialize the clients.

	client, err = teleport.NewClient(ctx, auth, s.userNames.requestor)
	require.NoError(t, err)
	s.clients[s.userNames.requestor] = client

	if teleportFeatures.AdvancedAccessWorkflows {
		client, err = teleport.NewClient(ctx, auth, s.userNames.approver)
		require.NoError(t, err)
		s.clients[s.userNames.approver] = client

		client, err = teleport.NewClient(ctx, auth, s.userNames.reviewer1)
		require.NoError(t, err)
		s.clients[s.userNames.reviewer1] = client

		client, err = teleport.NewClient(ctx, auth, s.userNames.reviewer2)
		require.NoError(t, err)
		s.clients[s.userNames.reviewer2] = client

		client, err = teleport.NewClient(ctx, auth, s.userNames.racer1)
		require.NoError(t, err)
		s.clients[s.userNames.racer1] = client

		client, err = teleport.NewClient(ctx, auth, s.userNames.racer2)
		require.NoError(t, err)
		s.clients[s.userNames.racer2] = client
	}

	identityPath, err := teleport.Sign(ctx, auth, s.userNames.plugin)
	require.NoError(t, err)

	s.teleportConfig.Addr = auth.AuthAddr().String()
	s.teleportConfig.Identity = identityPath
	s.teleportFeatures = teleportFeatures
}

func (s *OpsgenieSuite) SetupTest() {
	t := s.T()

	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)

	fakeOpsgenie := NewFakeOpsgenie(s.raceNumber)
	t.Cleanup(fakeOpsgenie.Close)
	s.fakeOpsgenie = fakeOpsgenie

	s.ogNotifyResponder = s.fakeOpsgenie.StoreResponder(Responder{
		Name: NotifyServiceName,
	})
	s.ogResponder1 = s.fakeOpsgenie.StoreResponder(Responder{
		Name: ResponderName1,
	})
	s.ogResponder2 = s.fakeOpsgenie.StoreResponder(Responder{
		Name: ResponderName2,
	})
	s.ogResponder3 = s.fakeOpsgenie.StoreResponder(Responder{
		Name: ResponderName3,
	})

	var conf Config
	conf.Teleport = s.teleportConfig
	conf.ClientConfig.APIEndpoint = s.fakeOpsgenie.URL()

	s.appConfig = conf
	s.currentRequestor = s.userNames.requestor
	s.SetContextTimeout(5 * time.Second)
}

func (s *OpsgenieSuite) startApp() {
	t := s.T()
	t.Helper()

	app, err := NewOpsgenieApp(s.Context(), &s.appConfig)
	require.NoError(t, err)

	s.StartApp(app)
}

func (s *OpsgenieSuite) ruler() *integration.Client {
	return s.clients[s.userNames.ruler]
}

func (s *OpsgenieSuite) requestor() *integration.Client {
	return s.clients[s.currentRequestor]
}

func (s *OpsgenieSuite) reviewer1() *integration.Client {
	return s.clients[s.userNames.reviewer1]
}

func (s *OpsgenieSuite) reviewer2() *integration.Client {
	return s.clients[s.userNames.reviewer2]
}

func (s *OpsgenieSuite) newAccessRequest() types.AccessRequest {
	t := s.T()
	t.Helper()

	req, err := types.NewAccessRequest(uuid.New().String(), s.currentRequestor, "editor")
	req.SetSystemAnnotations(map[string][]string{
		NotifyServiceAnnotation: {NotifyServiceName},
	})
	require.NoError(s.T(), err)
	return req
}

func (s *OpsgenieSuite) createAccessRequest() types.AccessRequest {
	t := s.T()
	t.Helper()

	req := s.newAccessRequest()
	out, err := s.requestor().CreateAccessRequestV2(s.Context(), req)
	require.NoError(t, err)
	return out
}

func (s *OpsgenieSuite) checkPluginData(reqID string, cond func(PluginData) bool) PluginData {
	t := s.T()
	t.Helper()

	for {
		rawData, err := s.ruler().PollAccessRequestPluginData(s.Context(), "opsgenie", reqID)
		require.NoError(t, err)
		if data := DecodePluginData(rawData); cond(data) {
			return data
		}
	}
}

func (s *OpsgenieSuite) TestAlertCreation() {
	t := s.T()

	s.startApp()

	req := s.createAccessRequest()
	pluginData := s.checkPluginData(req.GetName(), func(data PluginData) bool {
		return data.AlertID != ""
	})

	alert, err := s.fakeOpsgenie.CheckNewAlert(s.Context())
	require.NoError(t, err, "no new alerts stored")

	assert.Equal(t, alert.ID, pluginData.AlertID)
}

func (s *OpsgenieSuite) TestApproval() {
	t := s.T()

	s.startApp()

	req := s.createAccessRequest()

	alert, err := s.fakeOpsgenie.CheckNewAlert(s.Context())
	require.NoError(t, err, "no new alerts stored")

	err = s.ruler().ApproveAccessRequest(s.Context(), req.GetName(), "okay")
	require.NoError(t, err)

	note, err := s.fakeOpsgenie.CheckNewAlertNote(s.Context())
	require.NoError(t, err)
	assert.Equal(t, alert.ID, note.AlertID)
	assert.Contains(t, note.Note, "Access request has been approved")
	assert.Contains(t, note.Note, "Reason: okay")

	alertUpdate, err := s.fakeOpsgenie.CheckAlertUpdate(s.Context())
	require.NoError(t, err)
	assert.Equal(t, "resolved", alertUpdate.Status)
}

func (s *OpsgenieSuite) TestDenial() {
	t := s.T()

	s.startApp()

	req := s.createAccessRequest()

	alert, err := s.fakeOpsgenie.CheckNewAlert(s.Context())
	require.NoError(t, err, "no new alerts stored")

	err = s.ruler().DenyAccessRequest(s.Context(), req.GetName(), "not okay")
	require.NoError(t, err)

	note, err := s.fakeOpsgenie.CheckNewAlertNote(s.Context())
	require.NoError(t, err)
	assert.Equal(t, alert.ID, note.AlertID)
	assert.Contains(t, note.Note, "Access request has been denied")
	assert.Contains(t, note.Note, "Reason: not okay")

	alertUpdate, err := s.fakeOpsgenie.CheckAlertUpdate(s.Context())
	require.NoError(t, err)
	assert.Equal(t, "resolved", alertUpdate.Status)
}

func (s *OpsgenieSuite) TestReviewNotes() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	s.startApp()

	req := s.createAccessRequest()

	err := s.reviewer1().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer1,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	err = s.reviewer2().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer2,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	pluginData := s.checkPluginData(req.GetName(), func(data PluginData) bool {
		return data.AlertID != "" && data.ReviewsCount == 2
	})

	note, err := s.fakeOpsgenie.CheckNewAlertNote(s.Context())
	require.NoError(t, err)
	assert.Equal(t, pluginData.AlertID, note.AlertID)
	assert.Contains(t, note.Note, s.userNames.reviewer1+" reviewed the request", "note must contain a review author")
	assert.Contains(t, note.Note, "Resolution: APPROVED", "note must contain an approval resolution")
	assert.Contains(t, note.Note, "Reason: okay", "note must contain an approval reason")

	note, err = s.fakeOpsgenie.CheckNewAlertNote(s.Context())
	require.NoError(t, err)
	assert.Equal(t, pluginData.AlertID, note.AlertID)
	assert.Contains(t, note.Note, s.userNames.reviewer2+" reviewed the request", "note must contain a review author")
	assert.Contains(t, note.Note, "Resolution: APPROVED", "note must contain a approval resolution")
	assert.Contains(t, note.Note, "Reason: not okay", "note must contain a denial reason")
}

func (s *OpsgenieSuite) TestApprovalByReview() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	s.startApp()

	req := s.createAccessRequest()

	alert, err := s.fakeOpsgenie.CheckNewAlert(s.Context())
	require.NoError(t, err, "no new alerts stored")

	err = s.reviewer1().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer1,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	err = s.reviewer2().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer2,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "finally okay",
	})
	require.NoError(t, err)

	note, err := s.fakeOpsgenie.CheckNewAlertNote(s.Context())
	require.NoError(t, err)
	assert.Equal(t, alert.ID, note.AlertID)
	assert.Contains(t, note.Note, s.userNames.reviewer1+" reviewed the request", "note must contain a review author")

	note, err = s.fakeOpsgenie.CheckNewAlertNote(s.Context())
	require.NoError(t, err)
	assert.Equal(t, alert.ID, note.AlertID)
	assert.Contains(t, note.Note, s.userNames.reviewer2+" reviewed the request", "note must contain a review author")

	data := s.checkPluginData(req.GetName(), func(data PluginData) bool {
		return data.ReviewsCount == 2 && data.Resolution.Tag != Unresolved
	})
	assert.Equal(t, Resolution{Tag: ResolvedApproved, Reason: "finally okay"}, data.Resolution)

	note, err = s.fakeOpsgenie.CheckNewAlertNote(s.Context())
	require.NoError(t, err)
	assert.Equal(t, alert.ID, note.AlertID)
	assert.Contains(t, note.Note, "Access request has been approved")
	assert.Contains(t, note.Note, "Reason: finally okay")

	alertUpdate, err := s.fakeOpsgenie.CheckAlertUpdate(s.Context())
	require.NoError(t, err)
	assert.Equal(t, "resolved", alertUpdate.Status)
}

func (s *OpsgenieSuite) TestDenialByReview() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	s.startApp()

	req := s.createAccessRequest()

	alert, err := s.fakeOpsgenie.CheckNewAlert(s.Context())
	require.NoError(t, err, "no new alerts stored")

	err = s.reviewer1().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer1,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	err = s.reviewer2().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer2,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "finally not okay",
	})
	require.NoError(t, err)

	note, err := s.fakeOpsgenie.CheckNewAlertNote(s.Context())
	require.NoError(t, err)
	assert.Equal(t, alert.ID, note.AlertID)
	assert.Contains(t, note.Note, s.userNames.reviewer1+" reviewed the request", "note must contain a review author")

	note, err = s.fakeOpsgenie.CheckNewAlertNote(s.Context())
	require.NoError(t, err)
	assert.Equal(t, alert.ID, note.AlertID)
	assert.Contains(t, note.Note, s.userNames.reviewer2+" reviewed the request", "note must contain a review author")

	data := s.checkPluginData(req.GetName(), func(data PluginData) bool {
		return data.ReviewsCount == 2 && data.Resolution.Tag != Unresolved
	})
	assert.Equal(t, Resolution{Tag: ResolvedDenied, Reason: "finally not okay"}, data.Resolution)

	note, err = s.fakeOpsgenie.CheckNewAlertNote(s.Context())
	require.NoError(t, err)
	assert.Equal(t, alert.ID, note.AlertID)
	assert.Contains(t, note.Note, "Access request has been denied")
	assert.Contains(t, note.Note, "Reason: finally not okay")

	alertUpdate, err := s.fakeOpsgenie.CheckAlertUpdate(s.Context())
	require.NoError(t, err)
	assert.Equal(t, "resolved", alertUpdate.Status)
}
