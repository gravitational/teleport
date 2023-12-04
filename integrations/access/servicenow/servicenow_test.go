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

package servicenow

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
	ScheduleAnnotation = types.TeleportNamespace + types.ReqAnnotationSchedulesLabel
	Schedule           = "someRotaID"
	ResponderName1     = "ResponderID1"
	ResponderName2     = "RespondeID2"
	ResponderName3     = "RespondeID3"
)

type ServiceNowSuite struct {
	integration.Suite
	appConfig        Config
	currentRequestor string
	userNames        struct {
		ruler                  string
		reviewer1              string
		reviewer2              string
		requestor              string
		requestorWithSchedules string
		approver               string
		racer1                 string
		racer2                 string
		plugin                 string
	}
	raceNumber     int
	fakeServiceNow *FakeServiceNow

	snResponder1 string
	snResponder2 string
	snResponder3 string

	clients          map[string]*integration.Client
	teleportFeatures *proto.Features
	teleportConfig   lib.TeleportConfig
}

func TestServiceNowSuite(t *testing.T) { suite.Run(t, &ServiceNowSuite{}) }

func (s *ServiceNowSuite) SetupSuite() {
	var err error
	t := s.T()

	logger.Init()
	err = logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)
	s.raceNumber = 2 * runtime.GOMAXPROCS(0)
	me, err := user.Current()
	require.NoError(t, err)

	ctx := s.SetContextTimeout(30 * time.Second)

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
		},
	}
	if teleportFeatures.AdvancedAccessWorkflows {
		conditions.Request.Thresholds = []types.AccessReviewThreshold{{Approve: 2, Deny: 2}}
	}
	// This is the role for testing notification incident creation.
	role, err := bootstrap.AddRole("foo", types.RoleSpecV6{Allow: conditions})
	require.NoError(t, err)

	user, err := bootstrap.AddUserWithRoles(me.Username+"@example.com", role.GetName())
	require.NoError(t, err)
	s.userNames.requestor = user.GetName()

	// Set up user who can request the access to role "editor" but with schedule annotation.
	conditionsWithSchedule := types.RoleConditions{
		Request: &types.AccessRequestConditions{
			Roles: []string{"editor"},
			Annotations: wrappers.Traits{
				ScheduleAnnotation: []string{Schedule},
			},
		},
	}
	// This is the role for testing notification incident creation With schedule.
	roleWithSchedule, err := bootstrap.AddRole("fooWithSchedule", types.RoleSpecV6{Allow: conditionsWithSchedule})
	require.NoError(t, err)

	userWithSchedule, err := bootstrap.AddUserWithRoles(me.Username+"-schedule@example.com", roleWithSchedule.GetName())
	require.NoError(t, err)
	s.userNames.requestorWithSchedules = userWithSchedule.GetName()

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
		// It's handy to test auto-approval scenarios so we also set the schedule annotation.
		role, err = bootstrap.AddRole("bar", types.RoleSpecV6{
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles: []string{"editor"},
					Annotations: wrappers.Traits{
						ScheduleAnnotation: []string{Schedule},
					},
				},
			},
		})
		require.NoError(t, err)

		user, err = bootstrap.AddUserWithRoles(me.Username+"-approver@example.com", role.GetName())
		require.NoError(t, err)
		s.userNames.approver = user.GetName()

		role, err = bootstrap.AddRole("foo-bar", types.RoleSpecV6{
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles:      []string{"editor"},
					Thresholds: []types.AccessReviewThreshold{{Approve: 2, Deny: 2}},
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

	role, err = bootstrap.AddRole("access-servicenow", types.RoleSpecV6{Allow: conditions})
	require.NoError(t, err)

	user, err = bootstrap.AddUserWithRoles("access-servicenow", role.GetName())
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
		client, err = teleport.NewClient(ctx, auth, s.userNames.requestorWithSchedules)
		require.NoError(t, err)
		s.clients[s.userNames.requestorWithSchedules] = client

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

func (s *ServiceNowSuite) SetupTest() {
	t := s.T()

	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)

	fakeServiceNow := NewFakeServiceNow(s.raceNumber, s.userNames.requestorWithSchedules)
	t.Cleanup(fakeServiceNow.Close)
	s.fakeServiceNow = fakeServiceNow

	s.snResponder1 = s.fakeServiceNow.StoreResponder(s.Context(), ResponderName1)
	s.snResponder2 = s.fakeServiceNow.StoreResponder(s.Context(), ResponderName2)
	s.snResponder3 = s.fakeServiceNow.StoreResponder(s.Context(), ResponderName3)

	var conf Config
	conf.Teleport = s.teleportConfig
	conf.ClientConfig.APIEndpoint = s.fakeServiceNow.URL()
	conf.ClientConfig.CloseCode = "resolved"

	s.appConfig = conf
	s.currentRequestor = s.userNames.requestor
	s.SetContextTimeout(5 * time.Second)
}

func (s *ServiceNowSuite) startApp() {
	t := s.T()
	t.Helper()

	app, err := NewServiceNowApp(s.Context(), &s.appConfig)
	require.NoError(t, err)

	s.StartApp(app)
}

func (s *ServiceNowSuite) ruler() *integration.Client {
	return s.clients[s.userNames.ruler]
}

func (s *ServiceNowSuite) requestor() *integration.Client {
	return s.clients[s.currentRequestor]
}

func (s *ServiceNowSuite) reviewer1() *integration.Client {
	return s.clients[s.userNames.reviewer1]
}

func (s *ServiceNowSuite) reviewer2() *integration.Client {
	return s.clients[s.userNames.reviewer2]
}

func (s *ServiceNowSuite) newAccessRequest() types.AccessRequest {
	t := s.T()
	t.Helper()

	req, err := types.NewAccessRequest(uuid.New().String(), s.currentRequestor, "editor")
	require.NoError(s.T(), err)
	return req
}

func (s *ServiceNowSuite) createAccessRequest() types.AccessRequest {
	t := s.T()
	t.Helper()

	req := s.newAccessRequest()
	out, err := s.requestor().CreateAccessRequestV2(s.Context(), req)
	require.NoError(t, err)
	return out
}

func (s *ServiceNowSuite) checkPluginData(reqID string, cond func(PluginData) bool) PluginData {
	t := s.T()
	t.Helper()

	for {
		rawData, err := s.ruler().PollAccessRequestPluginData(s.Context(), "servicenow", reqID)
		require.NoError(t, err)
		data, err := DecodePluginData(rawData)
		require.NoError(t, err)
		if cond(data) {
			return data
		}
	}
}

func (s *ServiceNowSuite) TestIncidentCreation() {
	t := s.T()

	s.startApp()

	req := s.createAccessRequest()
	pluginData := s.checkPluginData(req.GetName(), func(data PluginData) bool {
		return data.IncidentID != ""
	})

	incident, err := s.fakeServiceNow.CheckNewIncident(s.Context())
	require.NoError(t, err, "no new incidents stored")

	assert.Equal(t, incident.IncidentID, pluginData.IncidentID)
}

func (s *ServiceNowSuite) TestApproval() {
	t := s.T()

	s.startApp()

	req := s.createAccessRequest()

	_, err := s.fakeServiceNow.CheckNewIncident(s.Context())
	require.NoError(t, err, "no new incidents stored")

	err = s.ruler().ApproveAccessRequest(s.Context(), req.GetName(), "okay")
	require.NoError(t, err)

	incident, err := s.fakeServiceNow.CheckIncidentUpdate(s.Context())
	require.NoError(t, err)
	require.Contains(t, incident.Description, "submitted access request")
	assert.Contains(t, incident.CloseNotes, "Access request has been resolved")
	assert.Contains(t, incident.CloseNotes, "Reason: okay")
	assert.Equal(t, "resolved", incident.CloseCode)
}

func (s *ServiceNowSuite) TestDenial() {
	t := s.T()

	s.startApp()

	req := s.createAccessRequest()

	incident, err := s.fakeServiceNow.CheckNewIncident(s.Context())
	require.NoError(t, err, "no new incidents stored")
	require.Contains(t, incident.Description, "submitted access request")

	err = s.ruler().DenyAccessRequest(s.Context(), req.GetName(), "not okay")
	require.NoError(t, err)

	incident, err = s.fakeServiceNow.CheckIncidentUpdate(s.Context())
	require.NoError(t, err)
	assert.Contains(t, incident.CloseNotes, "Access request has been resolved")
	assert.Contains(t, incident.CloseNotes, "Reason: not okay")
	require.NoError(t, err)
	assert.Equal(t, "resolved", incident.CloseCode)
}

func (s *ServiceNowSuite) TestReviewNotes() {
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

	_ = s.checkPluginData(req.GetName(), func(data PluginData) bool {
		return data.IncidentID != "" && data.ReviewsCount == 2
	})

	incident, err := s.fakeServiceNow.CheckIncidentUpdate(s.Context())
	require.NoError(t, err)
	assert.Contains(t, incident.WorkNotes, s.userNames.reviewer1+" reviewed the request", "note must contain a review author")
	assert.Contains(t, incident.WorkNotes, "Resolution: APPROVED", "note must contain an approval resolution")
	assert.Contains(t, incident.WorkNotes, "Reason: okay", "note must contain an approval reason")

	incident, err = s.fakeServiceNow.CheckIncidentUpdate(s.Context())
	require.NoError(t, err)
	assert.Contains(t, incident.WorkNotes, s.userNames.reviewer2+" reviewed the request", "note must contain a review author")
	assert.Contains(t, incident.WorkNotes, "Resolution: APPROVED", "note must contain a approval resolution")
	assert.Contains(t, incident.WorkNotes, "Reason: not okay", "note must contain a denial reason")
}

func (s *ServiceNowSuite) TestApprovalByReview() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	s.startApp()

	req := s.createAccessRequest()

	incident, err := s.fakeServiceNow.CheckNewIncident(s.Context())
	require.NoError(t, err, "no new incidents stored")
	require.Contains(t, incident.Description, "submitted access request")

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

	incident, err = s.fakeServiceNow.CheckIncidentUpdate(s.Context())
	require.NoError(t, err)
	assert.Contains(t, incident.WorkNotes, s.userNames.reviewer1+" reviewed the request", "note must contain a review author")

	incident, err = s.fakeServiceNow.CheckIncidentUpdate(s.Context())
	require.NoError(t, err)
	assert.Contains(t, incident.WorkNotes, s.userNames.reviewer2+" reviewed the request", "note must contain a review author")

	data := s.checkPluginData(req.GetName(), func(data PluginData) bool {
		return data.ReviewsCount == 2 && data.Resolution.State != ""
	})
	assert.Equal(t, Resolution{State: ResolutionStateResolved, Reason: "finally okay"}, data.Resolution)

	incident, err = s.fakeServiceNow.CheckIncidentUpdate(s.Context())
	require.NoError(t, err)
	assert.Contains(t, incident.CloseNotes, "Access request has been resolved")
	assert.Contains(t, incident.WorkNotes, "Reason: finally okay")
	assert.Equal(t, "resolved", incident.CloseCode)
}

func (s *ServiceNowSuite) TestDenialByReview() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	s.startApp()

	req := s.createAccessRequest()

	incident, err := s.fakeServiceNow.CheckNewIncident(s.Context())
	require.NoError(t, err, "no new incidents stored")
	require.Contains(t, incident.Description, "submitted access request")

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

	incident, err = s.fakeServiceNow.CheckIncidentUpdate(s.Context())
	require.NoError(t, err)
	assert.Contains(t, incident.WorkNotes, s.userNames.reviewer1+" reviewed the request", "note must contain a review author")

	incident, err = s.fakeServiceNow.CheckIncidentUpdate(s.Context())
	require.NoError(t, err)
	assert.Contains(t, incident.WorkNotes, s.userNames.reviewer2+" reviewed the request", "note must contain a review author")

	data := s.checkPluginData(req.GetName(), func(data PluginData) bool {
		return data.ReviewsCount == 2 && data.Resolution.State != ""
	})
	assert.Equal(t, Resolution{State: ResolutionStateClosed, Reason: "finally not okay"}, data.Resolution)

	incident, err = s.fakeServiceNow.CheckIncidentUpdate(s.Context())
	require.NoError(t, err)
	assert.Contains(t, incident.CloseNotes, "Access request has been resolved")
	assert.Contains(t, incident.CloseNotes, "Reason: finally not okay")
	assert.Equal(t, "resolved", incident.CloseCode)
}

func (s *ServiceNowSuite) TestAutoApproval() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	s.startApp()

	s.currentRequestor = s.userNames.requestorWithSchedules
	_ = s.createAccessRequest()

	_, err := s.fakeServiceNow.CheckNewIncident(s.Context())
	require.NoError(t, err, "no new incidents stored")

	incident, err := s.fakeServiceNow.CheckIncidentUpdate(s.Context())
	require.NoError(t, err)
	assert.Contains(t, incident.WorkNotes, "Resolution: APPROVED")
}
