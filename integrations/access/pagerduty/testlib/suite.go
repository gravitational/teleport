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

package testlib

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/pagerduty"
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

// PagerdutyBaseSuite is the Pagerduty access plugin test suite.
// It implements the testify.TestingSuite interface.
type PagerdutyBaseSuite struct {
	*integration.AccessRequestSuite
	appConfig     pagerduty.Config
	raceNumber    int
	fakePagerduty *FakePagerduty

	pdNotifyService pagerduty.Service
	pdService1      pagerduty.Service
	pdService2      pagerduty.Service
	pdService3      pagerduty.Service
}

// SetupTest starts a fake Pagerduty and generates the plugin configuration.
// It also configures the role notifications for Pagerduty notifications and
// automatic approval.
// It is run for each test.
func (s *PagerdutyBaseSuite) SetupTest() {
	t := s.T()
	ctx := context.Background()

	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)
	s.raceNumber = 2 * runtime.GOMAXPROCS(0)

	s.fakePagerduty = NewFakePagerduty(s.raceNumber)
	t.Cleanup(s.fakePagerduty.Close)

	// This service should be notified for every access request.
	s.pdNotifyService = s.fakePagerduty.StoreService(pagerduty.Service{
		Name: NotifyServiceName,
	})
	s.AnnotateRequesterRoleAccessRequests(
		ctx,
		pagerduty.NotifyServiceDefaultAnnotation,
		[]string{NotifyServiceName},
	)

	// Services 1 and 2 are configured to allow automatic approval if the
	// requesting user is on-call.
	s.pdService1 = s.fakePagerduty.StoreService(pagerduty.Service{
		Name:             ServiceName1,
		EscalationPolicy: pagerduty.Reference{Type: "escalation_policy_reference", ID: EscalationPolicyID1},
	})
	s.pdService2 = s.fakePagerduty.StoreService(pagerduty.Service{
		Name:             ServiceName2,
		EscalationPolicy: pagerduty.Reference{Type: "escalation_policy_reference", ID: EscalationPolicyID2},
	})
	s.AnnotateRequesterRoleAccessRequests(
		ctx,
		pagerduty.ServicesDefaultAnnotation,
		[]string{ServiceName1, ServiceName2},
	)

	// Service 3 should not trigger automatic approval.
	// It is here to test that a user on-call for another service is not granted access by mistake.
	s.pdService3 = s.fakePagerduty.StoreService(pagerduty.Service{
		Name:             ServiceName3,
		EscalationPolicy: pagerduty.Reference{Type: "escalation_policy_reference", ID: EscalationPolicyID3},
	})

	var conf pagerduty.Config
	conf.Teleport = s.TeleportConfig()
	conf.Pagerduty.APIEndpoint = s.fakePagerduty.URL()
	conf.Pagerduty.UserEmail = "bot@example.com"
	conf.Pagerduty.RequestAnnotations.NotifyService = pagerduty.NotifyServiceDefaultAnnotation
	conf.Pagerduty.RequestAnnotations.Services = pagerduty.ServicesDefaultAnnotation

	s.appConfig = conf
}

// startApp starts the Pagerduty plugin, waits for it to become ready and returns.
func (s *PagerdutyBaseSuite) startApp() {
	t := s.T()
	t.Helper()

	app, err := pagerduty.NewApp(s.appConfig)
	require.NoError(t, err)
	integration.RunAndWaitReady(t, app)
}

// PagerdutySuiteOSS contains all tests that support running against a Teleport
// OSS Server.
type PagerdutySuiteOSS struct {
	PagerdutyBaseSuite
}

// PagerdutySuiteEnterprise contains all tests that require a Teleport Enterprise
// to run.
type PagerdutySuiteEnterprise struct {
	PagerdutyBaseSuite
}

// SetupTest overrides PagerdutyBaseSuite.SetupTest to check the Teleport features
// before each test.
func (s *PagerdutySuiteEnterprise) SetupTest() {
	t := s.T()
	s.RequireAdvancedWorkflow(t)
	s.PagerdutyBaseSuite.SetupTest()
}

// TestIncidentCreation validates that an incident is created to the service
// specified in the role's annotation.
func (s *PagerdutySuiteOSS) TestIncidentCreation() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test execution: create an access request
	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)

	// Validate the incident has been created in Pagerduty and its ID is stored
	// in the plugin_data.
	pluginData := s.checkPluginData(ctx, req.GetName(), func(data pagerduty.PluginData) bool {
		return data.IncidentID != ""
	})

	incident, err := s.fakePagerduty.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")

	assert.Equal(t, incident.ID, pluginData.IncidentID)
	assert.Equal(t, s.pdNotifyService.ID, pluginData.ServiceID)

	assert.Equal(t, pagerduty.PdIncidentKeyPrefix+"/"+req.GetName(), incident.IncidentKey)
	assert.Equal(t, "triggered", incident.Status)
}

// TestApproval tests that when a request is approved, its corresponding incident
// is updated to reflect the new request state and a note is added to the incident.
func (s *PagerdutySuiteOSS) TestApproval() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its incident.
	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)

	incident, err := s.fakePagerduty.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")

	// Test execution: we approve the request
	err = s.Ruler().ApproveAccessRequest(ctx, req.GetName(), "okay")
	require.NoError(t, err)

	// Validating the plugin added a note to the incident explaining it got approved.
	note, err := s.fakePagerduty.CheckNewIncidentNote(ctx)
	require.NoError(t, err)
	assert.Equal(t, incident.ID, note.IncidentID)
	assert.Contains(t, note.Content, "Access request has been approved")
	assert.Contains(t, note.Content, "Reason: okay")

	// Validating the plugin resolved the incident.
	incidentUpdate, err := s.fakePagerduty.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Equal(t, "resolved", incidentUpdate.Status)
}

// TestDenial tests that when a request is denied, its corresponding incident
// is updated to reflect the new request state and a note is added to the incident.
func (s *PagerdutySuiteOSS) TestDenial() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its incident.
	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)

	incident, err := s.fakePagerduty.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")

	// Test execution: we deny the request
	err = s.Ruler().DenyAccessRequest(ctx, req.GetName(), "not okay")
	require.NoError(t, err)

	// Validating the plugin added a note to the incident explaining it got denied.
	note, err := s.fakePagerduty.CheckNewIncidentNote(ctx)
	require.NoError(t, err)
	assert.Equal(t, incident.ID, note.IncidentID)
	assert.Contains(t, note.Content, "Access request has been denied")
	assert.Contains(t, note.Content, "Reason: not okay")

	// Validating the plugin resolved the incident.
	incidentUpdate, err := s.fakePagerduty.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Equal(t, "resolved", incidentUpdate.Status)
}

// TestReviewNotes tests that incident notes are sent after the access request
// is reviewed. Each review should create a new note.
func (s *PagerdutySuiteEnterprise) TestReviewNotes() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request.
	req := s.CreateAccessRequest(ctx, integration.Requester1UserName, nil)

	// Test execution: we submit two reviews
	err := s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	// Validate incident notes were sent with the correct content.
	pluginData := s.checkPluginData(ctx, req.GetName(), func(data pagerduty.PluginData) bool {
		return data.IncidentID != "" && data.ReviewsCount == 2
	})

	note, err := s.fakePagerduty.CheckNewIncidentNote(ctx)
	require.NoError(t, err)
	assert.Equal(t, pluginData.IncidentID, note.IncidentID)
	assert.Contains(t, note.Content, integration.Reviewer1UserName+" reviewed the request", "note must contain a review author")
	assert.Contains(t, note.Content, "Resolution: APPROVED", "note must contain an approval resolution")
	assert.Contains(t, note.Content, "Reason: okay", "note must contain an approval reason")

	note, err = s.fakePagerduty.CheckNewIncidentNote(ctx)
	require.NoError(t, err)
	assert.Equal(t, pluginData.IncidentID, note.IncidentID)
	assert.Contains(t, note.Content, integration.Reviewer2UserName+" reviewed the request", "note must contain a review author")
	assert.Contains(t, note.Content, "Resolution: DENIED", "note must contain a denial resolution")
	assert.Contains(t, note.Content, "Reason: not okay", "note must contain a denial reason")
}

// TestApprovalByReview tests that the incident is annotated and resolved after the
// access request approval threshold is reached.
func (s *PagerdutySuiteEnterprise) TestApprovalByReview() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its incident.
	req := s.CreateAccessRequest(ctx, integration.Requester1UserName, nil)

	incident, err := s.fakePagerduty.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")

	// Test execution: we submit a review and validate that a note was created.
	err = s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	note, err := s.fakePagerduty.CheckNewIncidentNote(ctx)
	require.NoError(t, err)
	assert.Equal(t, incident.ID, note.IncidentID)
	assert.Contains(t, note.Content, integration.Reviewer1UserName+" reviewed the request", "note must contain a review author")

	// Test execution: we submit a second review and validate that a note was created.
	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "finally okay",
	})
	require.NoError(t, err)

	note, err = s.fakePagerduty.CheckNewIncidentNote(ctx)
	require.NoError(t, err)
	assert.Equal(t, incident.ID, note.IncidentID)
	assert.Contains(t, note.Content, integration.Reviewer2UserName+" reviewed the request", "note must contain a review author")

	// Validate the alert got resolved, and a final note was added describing the resolution.
	data := s.checkPluginData(ctx, req.GetName(), func(data pagerduty.PluginData) bool {
		return data.ReviewsCount == 2 && data.Resolution.Tag != pagerduty.Unresolved
	})
	assert.Equal(t, pagerduty.Resolution{Tag: pagerduty.ResolvedApproved, Reason: "finally okay"}, data.Resolution)

	note, err = s.fakePagerduty.CheckNewIncidentNote(ctx)
	require.NoError(t, err)
	assert.Equal(t, incident.ID, note.IncidentID)
	assert.Contains(t, note.Content, "Access request has been approved")
	assert.Contains(t, note.Content, "Reason: finally okay")

	incidentUpdate, err := s.fakePagerduty.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Equal(t, "resolved", incidentUpdate.Status)
}

// TestDenialByReview tests that the incident is annotated and resolved after the
// access request denial threshold is reached.
func (s *PagerdutySuiteEnterprise) TestDenialByReview() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its incident.
	req := s.CreateAccessRequest(ctx, integration.Requester1UserName, nil)

	incident, err := s.fakePagerduty.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")

	// Test execution: we submit a review and validate that a note was created.
	err = s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	note, err := s.fakePagerduty.CheckNewIncidentNote(ctx)
	require.NoError(t, err)
	assert.Equal(t, incident.ID, note.IncidentID)
	assert.Contains(t, note.Content, integration.Reviewer1UserName+" reviewed the request", "note must contain a review author")

	// Test execution: we submit a review and validate that a note was created.
	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "finally not okay",
	})
	require.NoError(t, err)

	note, err = s.fakePagerduty.CheckNewIncidentNote(ctx)
	require.NoError(t, err)
	assert.Equal(t, incident.ID, note.IncidentID)
	assert.Contains(t, note.Content, integration.Reviewer2UserName+" reviewed the request", "note must contain a review author")

	// Validate the alert got resolved, and a final note was added describing the resolution.
	data := s.checkPluginData(ctx, req.GetName(), func(data pagerduty.PluginData) bool {
		return data.ReviewsCount == 2 && data.Resolution.Tag != pagerduty.Unresolved
	})
	assert.Equal(t, pagerduty.Resolution{Tag: pagerduty.ResolvedDenied, Reason: "finally not okay"}, data.Resolution)

	note, err = s.fakePagerduty.CheckNewIncidentNote(ctx)
	require.NoError(t, err)
	assert.Equal(t, incident.ID, note.IncidentID)
	assert.Contains(t, note.Content, "Access request has been denied")
	assert.Contains(t, note.Content, "Reason: finally not okay")

	incidentUpdate, err := s.fakePagerduty.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Equal(t, "resolved", incidentUpdate.Status)
}

func (s *PagerdutyBaseSuite) assertNewEvent(ctx context.Context, watcher types.Watcher, opType types.OpType, resourceKind, resourceName string) types.Event {
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
	case <-ctx.Done():
		t.Error(t, "No events received", ctx.Err())
	}
	return ev
}

func (s *PagerdutyBaseSuite) assertNoNewEvents(ctx context.Context, watcher types.Watcher) {
	t := s.T()
	t.Helper()

	select {
	case ev := <-watcher.Events():
		t.Errorf("Unexpected event %#v", ev)
	case <-time.After(250 * time.Millisecond):
	case <-ctx.Done():
		t.Error(t, ctx.Err())
	}
}

func (s *PagerdutyBaseSuite) assertReviewSubmitted(ctx context.Context, userName string) {
	t := s.T()
	t.Helper()

	watcher, err := s.Ruler().NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{{Kind: types.KindAccessRequest}},
	})
	require.NoError(t, err)
	defer watcher.Close()

	_ = s.assertNewEvent(ctx, watcher, types.OpInit, "", "")

	request := s.CreateAccessRequest(ctx, userName, nil)
	reqID := request.GetName()

	ev := s.assertNewEvent(ctx, watcher, types.OpPut, types.KindAccessRequest, reqID)
	request, ok := ev.Resource.(types.AccessRequest)
	require.True(t, ok)
	assert.Empty(t, request.GetReviews())
	assert.Equal(t, types.RequestState_PENDING, request.GetState())

	ev = s.assertNewEvent(ctx, watcher, types.OpPut, types.KindAccessRequest, reqID)
	request, ok = ev.Resource.(types.AccessRequest)
	require.True(t, ok)
	assert.Equal(t, types.RequestState_APPROVED, request.GetState())
	reqReviews := request.GetReviews()
	assert.Len(t, reqReviews, 1)
	assert.Equal(t, integration.PluginUserName, reqReviews[0].Author)
}

func (s *PagerdutyBaseSuite) assertNoReviewSubmitted(ctx context.Context, userName string) {
	t := s.T()
	t.Helper()

	watcher, err := s.Ruler().NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{{Kind: types.KindAccessRequest}},
	})
	require.NoError(t, err)
	defer watcher.Close()

	_ = s.assertNewEvent(ctx, watcher, types.OpInit, "", "")

	request := s.CreateAccessRequest(ctx, userName, nil)
	reqID := request.GetName()

	ev := s.assertNewEvent(ctx, watcher, types.OpPut, types.KindAccessRequest, reqID)

	request, ok := ev.Resource.(types.AccessRequest)
	require.True(t, ok)
	assert.Equal(t, types.RequestState_PENDING, request.GetState())
	assert.Empty(t, request.GetReviews())

	s.assertNoNewEvents(ctx, watcher)

	request, err = s.Ruler().GetAccessRequest(ctx, request.GetName())
	require.NoError(t, err)
	assert.Equal(t, types.RequestState_PENDING, request.GetState())
	assert.Empty(t, request.GetReviews())
}

// TestAutoApprovalWhenNotOnCall tests that access requests are not automatically
// approved when the user is not on-call.
func (s *PagerdutySuiteEnterprise) TestAutoApprovalWhenNotOnCall() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// We use the OSS user for this advanced workflow test because their role
	// doesn't have a threshold
	userName := integration.RequesterOSSUserName
	s.fakePagerduty.StoreUser(pagerduty.User{
		Name:  "Test User",
		Email: userName,
	})
	s.startApp()
	s.assertNoReviewSubmitted(ctx, userName)
}

// TestAutoApprovalWhenOnCall tests that access requests are automatically
// approved when the user is on-call.
func (s *PagerdutySuiteEnterprise) TestAutoApprovalWhenOnCall() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// We use the OSS user for this advanced workflow test because their role
	// doesn't have a threshold
	userName := integration.RequesterOSSUserName
	pdUser := s.fakePagerduty.StoreUser(pagerduty.User{
		Name:  "Test User",
		Email: userName,
	})
	s.fakePagerduty.StoreOnCall(pagerduty.OnCall{
		User:             pagerduty.Reference{Type: "user_reference", ID: pdUser.ID},
		EscalationPolicy: pagerduty.Reference{Type: "escalation_policy_reference", ID: EscalationPolicyID1},
	})
	s.startApp()
	s.assertReviewSubmitted(ctx, userName)
}

// TestAutoApprovalWhenOnCallInSecondPolicy tests that access requests are
// automatically approved when the user is not on-call for the first service
// escalation policy but is on-call for the second service.
func (s *PagerdutySuiteEnterprise) TestAutoApprovalWhenOnCallInSecondPolicy() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// We use the OSS user for this advanced workflow test because their role
	// doesn't have a threshold
	userName := integration.RequesterOSSUserName
	pdUser := s.fakePagerduty.StoreUser(pagerduty.User{
		Name:  "Test User",
		Email: userName,
	})
	s.fakePagerduty.StoreOnCall(pagerduty.OnCall{
		User:             pagerduty.Reference{Type: "user_reference", ID: pdUser.ID},
		EscalationPolicy: pagerduty.Reference{Type: "escalation_policy_reference", ID: EscalationPolicyID2},
	})
	s.startApp()
	s.assertReviewSubmitted(ctx, userName)
}

// TestAutoApprovalWhenOnCallInSomeOtherPolicy tests that access requests are
// not automatically approved when the user is not on-call for a service
// specified in the role annotations, but is on-call for a third unrelated\
// service.
func (s *PagerdutySuiteEnterprise) TestAutoApprovalWhenOnCallInSomeOtherPolicy() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// We use the OSS user for this advanced workflow test because their role
	// doesn't have a threshold
	userName := integration.RequesterOSSUserName
	pdUser := s.fakePagerduty.StoreUser(pagerduty.User{
		Name:  "Test User",
		Email: userName,
	})
	s.fakePagerduty.StoreOnCall(pagerduty.OnCall{
		User:             pagerduty.Reference{Type: "user_reference", ID: pdUser.ID},
		EscalationPolicy: pagerduty.Reference{Type: "escalation_policy_reference", ID: EscalationPolicyID3},
	})
	s.startApp()
	s.assertNoReviewSubmitted(ctx, userName)
}

// TestExpiration tests that when a request expires, its corresponding incident
// is updated to reflect the new request state and a note is added to the incident.
func (s *PagerdutySuiteOSS) TestExpiration() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its incident.
	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)

	incident, err := s.fakePagerduty.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")
	assert.Equal(t, "triggered", incident.Status)
	incidentID := incident.ID

	s.checkPluginData(ctx, req.GetName(), func(data pagerduty.PluginData) bool {
		return data.IncidentID != ""
	})

	// Test execution: we expire the request
	err = s.Ruler().DeleteAccessRequest(ctx, req.GetName()) // simulate expiration
	require.NoError(t, err)

	// Validating the plugin resolved the incident and added a note explaining the reason.
	incident, err = s.fakePagerduty.CheckIncidentUpdate(ctx)
	require.NoError(t, err, "no new incidents updated")
	assert.Equal(t, incidentID, incident.ID)
	assert.Equal(t, "resolved", incident.Status)

	note, err := s.fakePagerduty.CheckNewIncidentNote(ctx)
	require.NoError(t, err, "no new notes stored")
	assert.Equal(t, incidentID, note.IncidentID)
	assert.Contains(t, note.Content, "Access request has been expired")
}

// TestRace validates that the plugin behaves properly and performs all the
// message updates when a lot of access requests are sent and reviewed in a very
// short time frame.
func (s *PagerdutySuiteEnterprise) TestRace() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	t.Cleanup(cancel)

	err := logger.Setup(logger.Config{Severity: "info"}) // Turn off noisy debug logging
	require.NoError(t, err)

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
	racer2 := s.fakePagerduty.StoreUser(pagerduty.User{
		Name:  "Mr Racer",
		Email: integration.Requester2UserName,
	})
	s.fakePagerduty.StoreOnCall(pagerduty.OnCall{
		User:             pagerduty.Reference{Type: "user_reference", ID: racer2.ID},
		EscalationPolicy: pagerduty.Reference{Type: "escalation_policy_reference", ID: EscalationPolicyID1},
	})

	watcher, err := s.Ruler().NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{{Kind: types.KindAccessRequest}},
	})
	require.NoError(t, err)
	defer watcher.Close()
	assert.Equal(t, types.OpInit, (<-watcher.Events()).Type)

	s.raceNumber = 1
	fmt.Printf("Race number: %d\n", s.raceNumber)
	process := lib.NewProcess(ctx)
	for i := 0; i < s.raceNumber; i++ {
		userName := integration.Requester1UserName
		var proposedState types.RequestState
		switch i % 2 {
		case 0:
			proposedState = types.RequestState_APPROVED
			userName = integration.Requester2UserName
		case 1:
			proposedState = types.RequestState_DENIED
		}
		process.SpawnCritical(func(ctx context.Context) error {
			req, err := types.NewAccessRequest(uuid.New().String(), userName, integration.RequestedRoleName)
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			req, err = s.ClientByName(userName).CreateAccessRequestV2(ctx, req)
			if err != nil {
				fmt.Printf("%s creates a request with error %s\n", userName, err)
				return setRaceErr(trace.Wrap(err))
			}
			fmt.Printf("%s creates a request without error\n", userName)
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
			req, err := s.Ruler().GetAccessRequest(ctx, reqID)
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}

			// All other requests must be resolved with either two approval reviews or two denial reviews.
			reviewsNumber := 2

			// Requests by racer2 must are auto-reviewed by plugin so only one approval is required.
			if req.GetUser() == integration.Requester2UserName {
				reviewsNumber = 1
				proposedState = types.RequestState_APPROVED
			}

			review := types.AccessReview{ProposedState: proposedState, Reason: "reviewed"}
			for j := 0; j < reviewsNumber; j++ {
				if j == 0 {
					review.Author = integration.Reviewer1UserName
				} else {
					review.Author = integration.Reviewer2UserName
				}
				review.Created = time.Now()
				if err = s.ClientByName(review.Author).SubmitAccessRequestReview(ctx, reqID, review); err != nil {
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

func getIncidentRequestID(incident pagerduty.Incident) (string, error) {
	prefix := pagerduty.PdIncidentKeyPrefix + "/"
	if !strings.HasPrefix(incident.IncidentKey, prefix) {
		return "", trace.Errorf("failed to parse incident_key %s", incident.IncidentKey)
	}
	return incident.IncidentKey[len(prefix):], nil
}
