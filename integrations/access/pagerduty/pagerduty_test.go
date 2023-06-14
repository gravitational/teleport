/*
Copyright 2020-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pagerduty

import (
	"context"
	"os/user"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
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
	EscalationPolicyID1 = "escalation_policy-1"
	EscalationPolicyID2 = "escalation_policy-2"
	EscalationPolicyID3 = "escalation_policy-3"
	NotifyServiceName   = "Teleport Notifications"
	ServiceName1        = "Service 1"
	ServiceName2        = "Service 2"
	ServiceName3        = "Service 3"
)

type PagerdutySuite struct {
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
	raceNumber      int
	fakePagerduty   *FakePagerduty
	pdNotifyService Service
	pdService1      Service
	pdService2      Service
	pdService3      Service

	clients          map[string]*integration.Client
	teleportFeatures *proto.Features
	teleportConfig   lib.TeleportConfig
}

func TestPagerdutySuite(t *testing.T) { suite.Run(t, &PagerdutySuite{}) }

func (s *PagerdutySuite) SetupSuite() {
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
				NotifyServiceDefaultAnnotation: []string{NotifyServiceName},
			},
		},
	}
	if teleportFeatures.AdvancedAccessWorkflows {
		conditions.Request.Thresholds = []types.AccessReviewThreshold{types.AccessReviewThreshold{Approve: 2, Deny: 2}}
	}
	// This is the role for testing notification incident creation.
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
		// It's handy to test auto-approval scenarios so we also put "pagerduty_services" annotation.
		role, err = bootstrap.AddRole("bar", types.RoleSpecV6{
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles: []string{"editor"},
					Annotations: wrappers.Traits{
						ServicesDefaultAnnotation: []string{ServiceName1, ServiceName2},
					},
				},
			},
		})
		require.NoError(t, err)

		user, err = bootstrap.AddUserWithRoles(me.Username+"-approver@example.com", role.GetName())
		require.NoError(t, err)
		s.userNames.approver = user.GetName()

		// This is the role with a maximum possible setup: both "pagerduty_notify_service" and
		// "pagerduty_services" annotations and threshold.
		role, err = bootstrap.AddRole("foo-bar", types.RoleSpecV6{
			Allow: types.RoleConditions{
				Request: &types.AccessRequestConditions{
					Roles: []string{"editor"},
					Annotations: wrappers.Traits{
						NotifyServiceDefaultAnnotation: []string{NotifyServiceName},
						ServicesDefaultAnnotation:      []string{ServiceName1, ServiceName2},
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

	role, err = bootstrap.AddRole("access-pagerduty", types.RoleSpecV6{Allow: conditions})
	require.NoError(t, err)

	user, err = bootstrap.AddUserWithRoles("access-pagerduty", role.GetName())
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

func (s *PagerdutySuite) SetupTest() {
	t := s.T()

	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)

	fakePagerduty := NewFakePagerduty(s.raceNumber)
	t.Cleanup(fakePagerduty.Close)
	s.fakePagerduty = fakePagerduty

	s.pdNotifyService = s.fakePagerduty.StoreService(Service{
		Name: NotifyServiceName,
	})
	s.pdService1 = s.fakePagerduty.StoreService(Service{
		Name:             ServiceName1,
		EscalationPolicy: Reference{Type: "escalation_policy_reference", ID: EscalationPolicyID1},
	})
	s.pdService2 = s.fakePagerduty.StoreService(Service{
		Name:             ServiceName2,
		EscalationPolicy: Reference{Type: "escalation_policy_reference", ID: EscalationPolicyID2},
	})
	s.pdService3 = s.fakePagerduty.StoreService(Service{
		Name:             ServiceName3,
		EscalationPolicy: Reference{Type: "escalation_policy_reference", ID: EscalationPolicyID3},
	})

	var conf Config
	conf.Teleport = s.teleportConfig
	conf.Pagerduty.APIEndpoint = s.fakePagerduty.URL()
	conf.Pagerduty.UserEmail = "bot@example.com"
	conf.Pagerduty.RequestAnnotations.NotifyService = NotifyServiceDefaultAnnotation
	conf.Pagerduty.RequestAnnotations.Services = ServicesDefaultAnnotation

	s.appConfig = conf
	s.currentRequestor = s.userNames.requestor
	s.SetContextTimeout(5 * time.Second)
}

func (s *PagerdutySuite) startApp() {
	t := s.T()
	t.Helper()

	app, err := NewApp(s.appConfig)
	require.NoError(t, err)

	s.StartApp(app)
}

func (s *PagerdutySuite) ruler() *integration.Client {
	return s.clients[s.userNames.ruler]
}

func (s *PagerdutySuite) requestor() *integration.Client {
	return s.clients[s.currentRequestor]
}

func (s *PagerdutySuite) reviewer1() *integration.Client {
	return s.clients[s.userNames.reviewer1]
}

func (s *PagerdutySuite) reviewer2() *integration.Client {
	return s.clients[s.userNames.reviewer2]
}

func (s *PagerdutySuite) newAccessRequest() types.AccessRequest {
	t := s.T()
	t.Helper()

	req, err := types.NewAccessRequest(uuid.New().String(), s.currentRequestor, "editor")
	require.NoError(s.T(), err)
	return req
}

func (s *PagerdutySuite) createAccessRequest() types.AccessRequest {
	t := s.T()
	t.Helper()

	req := s.newAccessRequest()
	err := s.requestor().CreateAccessRequest(s.Context(), req)
	require.NoError(t, err)
	return req
}

func (s *PagerdutySuite) checkPluginData(reqID string, cond func(PluginData) bool) PluginData {
	t := s.T()
	t.Helper()

	for {
		rawData, err := s.ruler().PollAccessRequestPluginData(s.Context(), "pagerduty", reqID)
		require.NoError(t, err)
		if data := DecodePluginData(rawData); cond(data) {
			return data
		}
	}
}

func (s *PagerdutySuite) TestIncidentCreation() {
	t := s.T()

	s.startApp()

	req := s.createAccessRequest()
	pluginData := s.checkPluginData(req.GetName(), func(data PluginData) bool {
		return data.IncidentID != ""
	})

	incident, err := s.fakePagerduty.CheckNewIncident(s.Context())
	require.NoError(t, err, "no new incidents stored")

	assert.Equal(t, incident.ID, pluginData.IncidentID)
	assert.Equal(t, s.pdNotifyService.ID, pluginData.ServiceID)

	assert.Equal(t, pdIncidentKeyPrefix+"/"+req.GetName(), incident.IncidentKey)
	assert.Equal(t, "triggered", incident.Status)
}

func (s *PagerdutySuite) TestApproval() {
	t := s.T()

	s.startApp()

	req := s.createAccessRequest()

	incident, err := s.fakePagerduty.CheckNewIncident(s.Context())
	require.NoError(t, err, "no new incidents stored")

	err = s.ruler().ApproveAccessRequest(s.Context(), req.GetName(), "okay")
	require.NoError(t, err)

	note, err := s.fakePagerduty.CheckNewIncidentNote(s.Context())
	require.NoError(t, err)
	assert.Equal(t, incident.ID, note.IncidentID)
	assert.Contains(t, note.Content, "Access request has been approved")
	assert.Contains(t, note.Content, "Reason: okay")

	incidentUpdate, err := s.fakePagerduty.CheckIncidentUpdate(s.Context())
	require.NoError(t, err)
	assert.Equal(t, "resolved", incidentUpdate.Status)
}

func (s *PagerdutySuite) TestDenial() {
	t := s.T()

	s.startApp()

	req := s.createAccessRequest()

	incident, err := s.fakePagerduty.CheckNewIncident(s.Context())
	require.NoError(t, err, "no new incidents stored")

	err = s.ruler().DenyAccessRequest(s.Context(), req.GetName(), "not okay")
	require.NoError(t, err)

	note, err := s.fakePagerduty.CheckNewIncidentNote(s.Context())
	require.NoError(t, err)
	assert.Equal(t, incident.ID, note.IncidentID)
	assert.Contains(t, note.Content, "Access request has been denied")
	assert.Contains(t, note.Content, "Reason: not okay")

	incidentUpdate, err := s.fakePagerduty.CheckIncidentUpdate(s.Context())
	require.NoError(t, err)
	assert.Equal(t, "resolved", incidentUpdate.Status)
}

func (s *PagerdutySuite) TestReviewNotes() {
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
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	pluginData := s.checkPluginData(req.GetName(), func(data PluginData) bool {
		return data.IncidentID != "" && data.ReviewsCount == 2
	})

	note, err := s.fakePagerduty.CheckNewIncidentNote(s.Context())
	require.NoError(t, err)
	assert.Equal(t, pluginData.IncidentID, note.IncidentID)
	assert.Contains(t, note.Content, s.userNames.reviewer1+" reviewed the request", "note must contain a review author")
	assert.Contains(t, note.Content, "Resolution: APPROVED", "note must contain an approval resolution")
	assert.Contains(t, note.Content, "Reason: okay", "note must contain an approval reason")

	note, err = s.fakePagerduty.CheckNewIncidentNote(s.Context())
	require.NoError(t, err)
	assert.Equal(t, pluginData.IncidentID, note.IncidentID)
	assert.Contains(t, note.Content, s.userNames.reviewer2+" reviewed the request", "note must contain a review author")
	assert.Contains(t, note.Content, "Resolution: DENIED", "note must contain a denial resolution")
	assert.Contains(t, note.Content, "Reason: not okay", "note must contain a denial reason")
}

func (s *PagerdutySuite) TestApprovalByReview() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	s.startApp()

	req := s.createAccessRequest()

	incident, err := s.fakePagerduty.CheckNewIncident(s.Context())
	require.NoError(t, err, "no new incidents stored")

	err = s.reviewer1().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer1,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	note, err := s.fakePagerduty.CheckNewIncidentNote(s.Context())
	require.NoError(t, err)
	assert.Equal(t, incident.ID, note.IncidentID)
	assert.Contains(t, note.Content, s.userNames.reviewer1+" reviewed the request", "note must contain a review author")

	err = s.reviewer2().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer2,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "finally okay",
	})
	require.NoError(t, err)

	note, err = s.fakePagerduty.CheckNewIncidentNote(s.Context())
	require.NoError(t, err)
	assert.Equal(t, incident.ID, note.IncidentID)
	assert.Contains(t, note.Content, s.userNames.reviewer2+" reviewed the request", "note must contain a review author")

	data := s.checkPluginData(req.GetName(), func(data PluginData) bool {
		return data.ReviewsCount == 2 && data.Resolution.Tag != Unresolved
	})
	assert.Equal(t, Resolution{Tag: ResolvedApproved, Reason: "finally okay"}, data.Resolution)

	note, err = s.fakePagerduty.CheckNewIncidentNote(s.Context())
	require.NoError(t, err)
	assert.Equal(t, incident.ID, note.IncidentID)
	assert.Contains(t, note.Content, "Access request has been approved")
	assert.Contains(t, note.Content, "Reason: finally okay")

	incidentUpdate, err := s.fakePagerduty.CheckIncidentUpdate(s.Context())
	require.NoError(t, err)
	assert.Equal(t, "resolved", incidentUpdate.Status)
}

func (s *PagerdutySuite) TestDenialByReview() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	s.startApp()

	req := s.createAccessRequest()

	incident, err := s.fakePagerduty.CheckNewIncident(s.Context())
	require.NoError(t, err, "no new incidents stored")

	err = s.reviewer1().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer1,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	note, err := s.fakePagerduty.CheckNewIncidentNote(s.Context())
	require.NoError(t, err)
	assert.Equal(t, incident.ID, note.IncidentID)
	assert.Contains(t, note.Content, s.userNames.reviewer1+" reviewed the request", "note must contain a review author")

	err = s.reviewer2().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer2,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "finally not okay",
	})
	require.NoError(t, err)

	note, err = s.fakePagerduty.CheckNewIncidentNote(s.Context())
	require.NoError(t, err)
	assert.Equal(t, incident.ID, note.IncidentID)
	assert.Contains(t, note.Content, s.userNames.reviewer2+" reviewed the request", "note must contain a review author")

	data := s.checkPluginData(req.GetName(), func(data PluginData) bool {
		return data.ReviewsCount == 2 && data.Resolution.Tag != Unresolved
	})
	assert.Equal(t, Resolution{Tag: ResolvedDenied, Reason: "finally not okay"}, data.Resolution)

	note, err = s.fakePagerduty.CheckNewIncidentNote(s.Context())
	require.NoError(t, err)
	assert.Equal(t, incident.ID, note.IncidentID)
	assert.Contains(t, note.Content, "Access request has been denied")
	assert.Contains(t, note.Content, "Reason: finally not okay")

	incidentUpdate, err := s.fakePagerduty.CheckIncidentUpdate(s.Context())
	require.NoError(t, err)
	assert.Equal(t, "resolved", incidentUpdate.Status)
}

func (s *PagerdutySuite) assertNewEvent(watcher types.Watcher, opType types.OpType, resourceKind, resourceName string) types.Event {
	t := s.T()
	t.Helper()

	var ev types.Event
	select {
	case ev = <-watcher.Events():
		assert.Equal(t, opType, ev.Type)
		if resourceKind != "" {
			assert.Equal(t, resourceKind, ev.Resource.GetKind())
			assert.Equal(t, resourceName, ev.Resource.GetName())
		} else {
			switch r := ev.Resource.(type) {
			case nil:
			case *types.WatchStatusV1:
			default:
				t.Errorf("expected nil or *WatchStatusV1, got %T instead", r)
			}
		}
	case <-s.Context().Done():
		t.Error(t, "No events received", s.Context().Err())
	}
	return ev
}

func (s *PagerdutySuite) assertNoNewEvents(watcher types.Watcher) {
	t := s.T()
	t.Helper()

	select {
	case ev := <-watcher.Events():
		t.Errorf("Unexpected event %#v", ev)
	case <-time.After(250 * time.Millisecond):
	case <-s.Context().Done():
		t.Error(t, s.Context().Err())
	}
}

func (s *PagerdutySuite) assertReviewSubmitted() {
	t := s.T()
	t.Helper()

	watcher, err := s.ruler().NewWatcher(s.Context(), types.Watch{
		Kinds: []types.WatchKind{{Kind: types.KindAccessRequest}},
	})
	require.NoError(t, err)
	defer watcher.Close()

	_ = s.assertNewEvent(watcher, types.OpInit, "", "")

	request := s.createAccessRequest()
	reqID := request.GetName()

	ev := s.assertNewEvent(watcher, types.OpPut, types.KindAccessRequest, reqID)
	request, ok := ev.Resource.(types.AccessRequest)
	require.True(t, ok)
	assert.Len(t, request.GetReviews(), 0)
	assert.Equal(t, types.RequestState_PENDING, request.GetState())

	ev = s.assertNewEvent(watcher, types.OpPut, types.KindAccessRequest, reqID)
	request, ok = ev.Resource.(types.AccessRequest)
	require.True(t, ok)
	assert.Equal(t, types.RequestState_APPROVED, request.GetState())
	reqReviews := request.GetReviews()
	assert.Len(t, reqReviews, 1)
	assert.Equal(t, s.userNames.plugin, reqReviews[0].Author)
}

func (s *PagerdutySuite) assertNoReviewSubmitted() {
	t := s.T()
	t.Helper()

	watcher, err := s.ruler().NewWatcher(s.Context(), types.Watch{
		Kinds: []types.WatchKind{{Kind: types.KindAccessRequest}},
	})
	require.NoError(t, err)
	defer watcher.Close()

	_ = s.assertNewEvent(watcher, types.OpInit, "", "")

	request := s.createAccessRequest()
	reqID := request.GetName()

	ev := s.assertNewEvent(watcher, types.OpPut, types.KindAccessRequest, reqID)

	request, ok := ev.Resource.(types.AccessRequest)
	require.True(t, ok)
	assert.Equal(t, types.RequestState_PENDING, request.GetState())
	assert.Len(t, request.GetReviews(), 0)

	s.assertNoNewEvents(watcher)

	request, err = s.ruler().GetAccessRequest(s.Context(), request.GetName())
	require.NoError(t, err)
	assert.Equal(t, types.RequestState_PENDING, request.GetState())
	assert.Len(t, request.GetReviews(), 0)
}

func (s *PagerdutySuite) TestAutoApprovalWhenNotOnCall() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	s.currentRequestor = s.userNames.approver
	s.fakePagerduty.StoreUser(User{
		Name:  "Test User",
		Email: s.currentRequestor,
	})
	s.startApp()
	s.assertNoReviewSubmitted()
}

func (s *PagerdutySuite) TestAutoApprovalWhenOnCall() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	s.currentRequestor = s.userNames.approver
	pdUser := s.fakePagerduty.StoreUser(User{
		Name:  "Test User",
		Email: s.currentRequestor,
	})
	s.fakePagerduty.StoreOnCall(OnCall{
		User:             Reference{Type: "user_reference", ID: pdUser.ID},
		EscalationPolicy: Reference{Type: "escalation_policy_reference", ID: EscalationPolicyID1},
	})
	s.startApp()
	s.assertReviewSubmitted()
}

func (s *PagerdutySuite) TestAutoApprovalWhenOnCallInSecondPolicy() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	s.currentRequestor = s.userNames.approver
	pdUser := s.fakePagerduty.StoreUser(User{
		Name:  "Test User",
		Email: s.currentRequestor,
	})
	s.fakePagerduty.StoreOnCall(OnCall{
		User:             Reference{Type: "user_reference", ID: pdUser.ID},
		EscalationPolicy: Reference{Type: "escalation_policy_reference", ID: EscalationPolicyID2},
	})
	s.startApp()
	s.assertReviewSubmitted()
}

func (s *PagerdutySuite) TestAutoApprovalWhenOnCallInSomeOtherPolicy() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	s.currentRequestor = s.userNames.approver
	pdUser := s.fakePagerduty.StoreUser(User{
		Name:  "Test User",
		Email: s.currentRequestor,
	})
	s.fakePagerduty.StoreOnCall(OnCall{
		User:             Reference{Type: "user_reference", ID: pdUser.ID},
		EscalationPolicy: Reference{Type: "escalation_policy_reference", ID: EscalationPolicyID3},
	})
	s.startApp()
	s.assertNoReviewSubmitted()
}

func (s *PagerdutySuite) TestExpiration() {
	t := s.T()

	s.startApp()

	req := s.createAccessRequest()

	incident, err := s.fakePagerduty.CheckNewIncident(s.Context())
	require.NoError(t, err, "no new incidents stored")
	assert.Equal(t, "triggered", incident.Status)
	incidentID := incident.ID

	s.checkPluginData(req.GetName(), func(data PluginData) bool {
		return data.IncidentID != ""
	})

	err = s.ruler().DeleteAccessRequest(s.Context(), req.GetName()) // simulate expiration
	require.NoError(t, err)

	incident, err = s.fakePagerduty.CheckIncidentUpdate(s.Context())
	require.NoError(t, err, "no new incidents updated")
	assert.Equal(t, incidentID, incident.ID)
	assert.Equal(t, "resolved", incident.Status)

	note, err := s.fakePagerduty.CheckNewIncidentNote(s.Context())
	require.NoError(t, err, "no new notes stored")
	assert.Equal(t, incidentID, note.IncidentID)
	assert.Contains(t, note.Content, "Access request has been expired")
}

func (s *PagerdutySuite) TestRace() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	err := logger.Setup(logger.Config{Severity: "info"}) // Turn off noisy debug logging
	require.NoError(t, err)

	s.SetContextTimeout(20 * time.Second)
	s.startApp()

	var (
		raceErr               error
		raceErrOnce           sync.Once
		pendingRequests       sync.Map
		resolvedRequests      sync.Map
		incidentIDs           sync.Map
		incidentsCount        int32
		incidentNoteCounters  sync.Map
		resolvedRequestsCount int32
	)
	setRaceErr := func(err error) error {
		raceErrOnce.Do(func() {
			raceErr = err
		})
		return err
	}

	// Set one of the users on-call and assign an incident to her.
	racer2 := s.fakePagerduty.StoreUser(User{
		Name:  "Mr Racer",
		Email: s.userNames.racer2,
	})
	s.fakePagerduty.StoreOnCall(OnCall{
		User:             Reference{Type: "user_reference", ID: racer2.ID},
		EscalationPolicy: Reference{Type: "escalation_policy_reference", ID: EscalationPolicyID1},
	})

	watcher, err := s.ruler().NewWatcher(s.Context(), types.Watch{
		Kinds: []types.WatchKind{{Kind: types.KindAccessRequest}},
	})
	require.NoError(t, err)
	defer watcher.Close()
	assert.Equal(t, types.OpInit, (<-watcher.Events()).Type)

	process := lib.NewProcess(s.Context())
	for i := 0; i < s.raceNumber; i++ {
		userName := s.userNames.racer1
		var proposedState types.RequestState
		switch i % 2 {
		case 0:
			proposedState = types.RequestState_APPROVED
			userName = s.userNames.racer2
		case 1:
			proposedState = types.RequestState_DENIED
		}
		process.SpawnCritical(func(ctx context.Context) error {
			req, err := types.NewAccessRequest(uuid.New().String(), userName, "editor")
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			if err := s.clients[userName].CreateAccessRequest(ctx, req); err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			pendingRequests.Store(req.GetName(), struct{}{})
			return nil
		})
		process.SpawnCritical(func(ctx context.Context) error {
			incident, err := s.fakePagerduty.CheckNewIncident(ctx)
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			if obtained, expected := incident.Status, "triggered"; obtained != expected {
				return setRaceErr(trace.Errorf("wrong incident status. expected %s, obtained %s", expected, obtained))
			}
			reqID, err := getIncidentRequestID(incident)
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			if _, loaded := incidentIDs.LoadOrStore(incident.ID, struct{}{}); loaded {
				return setRaceErr(trace.Errorf("incident %s has already been stored", incident.ID))
			}
			atomic.AddInt32(&incidentsCount, 1)
			req, err := s.ruler().GetAccessRequest(ctx, reqID)
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}

			// All other requests must be resolved with either two approval reviews or two denial reviews.
			reviewsNumber := 2

			// Requests by racer2 must are auto-reviewed by plugin so only one approval is required.
			if req.GetUser() == s.userNames.racer2 {
				reviewsNumber = 1
				proposedState = types.RequestState_APPROVED
			}

			review := types.AccessReview{ProposedState: proposedState, Reason: "reviewed"}
			for j := 0; j < reviewsNumber; j++ {
				if j == 0 {
					review.Author = s.userNames.reviewer1
				} else {
					review.Author = s.userNames.reviewer2
				}
				review.Created = time.Now()
				if err = s.clients[review.Author].SubmitAccessRequestReview(ctx, reqID, review); err != nil {
					return setRaceErr(trace.Wrap(err))
				}
			}
			return nil
		})
		process.SpawnCritical(func(ctx context.Context) error {
			incident, err := s.fakePagerduty.CheckIncidentUpdate(ctx)
			if err := trace.Wrap(err); err != nil {
				return setRaceErr(err)
			}
			if obtained, expected := incident.Status, "resolved"; obtained != expected {
				return setRaceErr(trace.Errorf("wrong incident status. expected %s, obtained %s", expected, obtained))
			}
			return nil
		})
	}
	for i := 0; i < 3*s.raceNumber; i++ {
		process.SpawnCritical(func(ctx context.Context) error {
			note, err := s.fakePagerduty.CheckNewIncidentNote(ctx)
			if err := trace.Wrap(err); err != nil {
				return setRaceErr(err)
			}

			var newCounter int32
			val, _ := incidentNoteCounters.LoadOrStore(note.IncidentID, &newCounter)
			counterPtr := val.(*int32)
			atomic.AddInt32(counterPtr, 1)

			return nil
		})
	}
	process.SpawnCritical(func(ctx context.Context) error {
		for {
			var event types.Event
			select {
			case event = <-watcher.Events():
			case <-ctx.Done():
				return setRaceErr(trace.Wrap(ctx.Err()))
			}
			if obtained, expected := event.Type, types.OpPut; obtained != expected {
				return setRaceErr(trace.Errorf("wrong event type. expected %v, obtained %v", expected, obtained))
			}
			if obtained, expected := event.Resource.GetKind(), types.KindAccessRequest; obtained != expected {
				return setRaceErr(trace.Errorf("wrong resource kind. expected %v, obtained %v", expected, obtained))
			}
			req := event.Resource.(types.AccessRequest)
			if req.GetState() != types.RequestState_APPROVED && req.GetState() != types.RequestState_DENIED {
				continue
			}
			resolvedRequests.Store(req.GetName(), struct{}{})
			if atomic.AddInt32(&resolvedRequestsCount, 1) == int32(s.raceNumber) {
				return nil
			}
		}
	})
	process.Terminate()
	<-process.Done()
	require.NoError(t, raceErr)

	pendingRequests.Range(func(key, _ interface{}) bool {
		_, ok := resolvedRequests.LoadAndDelete(key)
		return assert.True(t, ok)
	})

	assert.Equal(t, int32(s.raceNumber), resolvedRequestsCount)

	incidentIDs.Range(func(key, _ interface{}) bool {
		next := true

		val, ok := incidentNoteCounters.LoadAndDelete(key)
		next = next && assert.True(t, ok)
		counterPtr := val.(*int32)
		next = next && assert.Equal(t, int32(3), *counterPtr)

		return next
	})

	assert.Equal(t, int32(s.raceNumber), incidentsCount)
}

func getIncidentRequestID(incident Incident) (string, error) {
	prefix := pdIncidentKeyPrefix + "/"
	if !strings.HasPrefix(incident.IncidentKey, prefix) {
		return "", trace.Errorf("failed to parse incident_key %s", incident.IncidentKey)
	}
	return incident.IncidentKey[len(prefix):], nil
}
