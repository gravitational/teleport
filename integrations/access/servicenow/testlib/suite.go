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
	"runtime"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/servicenow"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
)

const snowOnCallRotationName = "important-rotation"

// ServiceNowSuite is the ServiceNow access plugin test suite.
// It implements the testify.TestingSuite interface.
type ServiceNowSuite struct {
	*integration.AccessRequestSuite
	appConfig      servicenow.Config
	raceNumber     int
	fakeServiceNow *FakeServiceNow

	snowUser1 string
	snowUser2 string
}

// SetupTest starts a fake ServiceNow and generates the plugin configuration.
// It also configures the role notifications for ServiceNow notifications and
// automatic approval.
// It is run for each test.
func (s *ServiceNowSuite) SetupTest() {
	t := s.T()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)
	s.raceNumber = 2 * runtime.GOMAXPROCS(0)

	s.fakeServiceNow = NewFakeServiceNow(s.raceNumber)
	t.Cleanup(s.fakeServiceNow.Close)

	s.snowUser1 = s.fakeServiceNow.StoreUser(integration.RequesterOSSUserName)
	s.snowUser2 = s.fakeServiceNow.StoreUser("some random other user")

	s.fakeServiceNow.StoreOnCall(snowOnCallRotationName, []string{})

	s.AnnotateRequesterRoleAccessRequests(
		ctx,
		types.TeleportNamespace+types.ReqAnnotationApproveSchedulesLabel,
		[]string{snowOnCallRotationName},
	)

	var conf servicenow.Config
	conf.Teleport = s.TeleportConfig()
	conf.ClientConfig.APIEndpoint = s.fakeServiceNow.URL()
	conf.ClientConfig.CloseCode = "resolved"

	s.appConfig = conf
}

// startApp starts the ServiceNow plugin, waits for it to become ready and returns.
func (s *ServiceNowSuite) startApp() {
	t := s.T()
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	app, err := servicenow.NewServiceNowApp(ctx, &s.appConfig)
	require.NoError(t, err)
	s.RunAndWaitReady(t, app)
}

// TestIncidentCreation validates that a new access request triggers an
// incident creation.
func (s *ServiceNowSuite) TestIncidentCreation() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test execution: we create a new access request.
	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)
	pluginData := s.checkPluginData(ctx, req.GetName(), func(data servicenow.PluginData) bool {
		return data.IncidentID != ""
	})

	// Validating a new incident was created.
	incident, err := s.fakeServiceNow.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")

	assert.Equal(t, incident.IncidentID, pluginData.IncidentID)
}

// TestApproval tests that when a request is approved, its corresponding incident
// is updated to reflect the new request state and a note is added to the incident.
func (s *ServiceNowSuite) TestApproval() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its incident.
	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)

	_, err := s.fakeServiceNow.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")

	// Test execution: we approve the request
	err = s.Ruler().ApproveAccessRequest(ctx, req.GetName(), "okay")
	require.NoError(t, err)

	// Validating the plugin closed the incident and explained the reason in the
	// close notes.
	incident, err := s.fakeServiceNow.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	require.Contains(t, incident.Description, "submitted access request")
	assert.Contains(t, incident.CloseNotes, "Access request has been resolved")
	assert.Contains(t, incident.CloseNotes, "Reason: okay")
	assert.Equal(t, "resolved", incident.CloseCode)
}

// TestDenial tests that when a request is denied, its corresponding incident
// is updated to reflect the new request state and a note is added to the incident.
func (s *ServiceNowSuite) TestDenial() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its incident.
	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)

	incident, err := s.fakeServiceNow.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")
	require.Contains(t, incident.Description, "submitted access request")

	// Test execution: we deny the request
	err = s.Ruler().DenyAccessRequest(ctx, req.GetName(), "not okay")
	require.NoError(t, err)

	// Validating the plugin closed the incident and explained the reason in the
	// close notes.
	incident, err = s.fakeServiceNow.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Contains(t, incident.CloseNotes, "Access request has been resolved")
	assert.Contains(t, incident.CloseNotes, "Reason: not okay")
	require.NoError(t, err)
	assert.Equal(t, "resolved", incident.CloseCode)
}

// TestReviewNotes tests that incident notes are sent after the access request
// is reviewed. Each review should create a new note.
func (s *ServiceNowSuite) TestReviewNotes() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	if !s.TeleportFeatures().AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

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
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	_ = s.checkPluginData(ctx, req.GetName(), func(data servicenow.PluginData) bool {
		return data.IncidentID != "" && data.ReviewsCount == 2
	})

	// Validate incident notes were sent with the correct content.
	incident, err := s.fakeServiceNow.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Contains(t, incident.WorkNotes, integration.Reviewer1UserName+" reviewed the request", "note must contain a review author")
	assert.Contains(t, incident.WorkNotes, "Resolution: APPROVED", "note must contain an approval resolution")
	assert.Contains(t, incident.WorkNotes, "Reason: okay", "note must contain an approval reason")

	incident, err = s.fakeServiceNow.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Contains(t, incident.WorkNotes, integration.Reviewer2UserName+" reviewed the request", "note must contain a review author")
	assert.Contains(t, incident.WorkNotes, "Resolution: APPROVED", "note must contain a approval resolution")
	assert.Contains(t, incident.WorkNotes, "Reason: not okay", "note must contain a denial reason")
}

// TestApprovalByReview tests that the incident is annotated and resolved after the
// access request approval threshold is reached.
func (s *ServiceNowSuite) TestApprovalByReview() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	if !s.TeleportFeatures().AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	s.startApp()

	// Test setup: we create an access request and wait for its incident.
	req := s.CreateAccessRequest(ctx, integration.Requester1UserName, nil)

	incident, err := s.fakeServiceNow.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")
	require.Contains(t, incident.Description, "submitted access request")

	// Test execution: we submit two reviews.
	err = s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "finally okay",
	})
	require.NoError(t, err)

	// Validate the incident is updated for each review.
	incident, err = s.fakeServiceNow.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Contains(t, incident.WorkNotes, integration.Reviewer1UserName+" reviewed the request", "note must contain a review author")

	incident, err = s.fakeServiceNow.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Contains(t, incident.WorkNotes, integration.Reviewer2UserName+" reviewed the request", "note must contain a review author")

	// Validate the plugin closed the incident and explained the reason in the
	// close notes.
	data := s.checkPluginData(ctx, req.GetName(), func(data servicenow.PluginData) bool {
		return data.ReviewsCount == 2 && data.Resolution.State != ""
	})
	assert.Equal(t, servicenow.Resolution{State: servicenow.ResolutionStateResolved, Reason: "finally okay"}, data.Resolution)

	incident, err = s.fakeServiceNow.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Contains(t, incident.CloseNotes, "Access request has been resolved")
	assert.Contains(t, incident.WorkNotes, "Reason: finally okay")
	assert.Equal(t, "resolved", incident.CloseCode)
}

// TestDenialByReview tests that the incident is annotated and resolved after the
// access request denial threshold is reached.
func (s *ServiceNowSuite) TestDenialByReview() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	if !s.TeleportFeatures().AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	s.startApp()

	// Test setup: we create an access request and wait for its incident.
	req := s.CreateAccessRequest(ctx, integration.Requester1UserName, nil)

	incident, err := s.fakeServiceNow.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")
	require.Contains(t, incident.Description, "submitted access request")

	// Test execution: we submit two reviews.
	err = s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "finally not okay",
	})
	require.NoError(t, err)

	// Validate the incident is updated for each review.
	incident, err = s.fakeServiceNow.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Contains(t, incident.WorkNotes, integration.Reviewer1UserName+" reviewed the request", "note must contain a review author")

	incident, err = s.fakeServiceNow.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Contains(t, incident.WorkNotes, integration.Reviewer2UserName+" reviewed the request", "note must contain a review author")

	// Validate the plugin closed the incident and explained the reason in the
	// close notes.
	data := s.checkPluginData(ctx, req.GetName(), func(data servicenow.PluginData) bool {
		return data.ReviewsCount == 2 && data.Resolution.State != ""
	})
	assert.Equal(t, servicenow.Resolution{State: servicenow.ResolutionStateClosed, Reason: "finally not okay"}, data.Resolution)

	incident, err = s.fakeServiceNow.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Contains(t, incident.CloseNotes, "Access request has been resolved")
	assert.Contains(t, incident.CloseNotes, "Reason: finally not okay")
	assert.Equal(t, "resolved", incident.CloseCode)
}

// TestAutoApproval tests that access requests are automatically
// approved when the user is on-call.
func (s *ServiceNowSuite) TestAutoApproval() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	if !s.TeleportFeatures().AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	s.startApp()

	// Test setup: put 2 users on-call for the requested rota.
	// User 1 is the requester while user 2 is another user, not in Teleport.
	s.fakeServiceNow.StoreOnCall(snowOnCallRotationName, []string{s.snowUser1, s.snowUser2})

	// Test execution: create the access request
	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)

	// Validate the incident was created and resolved.
	_, err := s.fakeServiceNow.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")

	incident, err := s.fakeServiceNow.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Contains(t, incident.WorkNotes, "Resolution: APPROVED")

	// Validate the request was reviewed and approved.
	req, err = s.Ruler().GetAccessRequest(ctx, req.GetName())
	require.NoError(t, err)
	require.Len(t, req.GetReviews(), 1, "request was not reviewed")
	require.Equal(t, types.RequestState_APPROVED, req.GetState(), "request was not approved")
}
