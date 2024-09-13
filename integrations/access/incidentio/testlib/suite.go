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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/incidentio"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	NotifyScheduleAnnotation   = types.TeleportNamespace + types.ReqAnnotationNotifySchedulesLabel
	ApprovalScheduleName       = "Teleport Approval"
	ApprovalScheduleAnnotation = types.TeleportNamespace + types.ReqAnnotationApproveSchedulesLabel
)

// IncidentBaseSuite is the incident.io access plugin test suite.
// It implements the testify.TestingSuite interface.
type IncidentBaseSuite struct {
	*integration.AccessRequestSuite
	appConfig    incidentio.Config
	raceNumber   int
	fakeIncident *FakeIncident

	incSchedule incidentio.ScheduleResult
}

// SetupTest starts a fake incident.io and generates the plugin configuration.
// It also configures the role notifications for incident.io notifications and
// automatic approval.
// It is run for each test.
func (s *IncidentBaseSuite) SetupTest() {
	t := s.T()
	ctx := context.Background()

	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)
	s.raceNumber = 2 * runtime.GOMAXPROCS(0)

	s.fakeIncident = NewFakeIncident(s.raceNumber)
	t.Cleanup(s.fakeIncident.Close)

	// This service should be notified for every access request.
	s.incSchedule = s.fakeIncident.StoreSchedule("aScheduleID", incidentio.ScheduleResult{
		Annotations: nil,
		Config: incidentio.ScheduleConfig{
			Rotations: nil,
		},
		CreatedAt:     time.Time{},
		CurrentShifts: nil,
		HolidaysPublicConfig: incidentio.HolidaysPublicConfig{
			CountryCodes: nil,
		},
		ID:        "aScheduleID",
		Name:      "Teleport Notifications One",
		Timezone:  "",
		UpdatedAt: time.Time{},
	})

	s.AnnotateRequesterRoleAccessRequests(
		ctx,
		NotifyScheduleAnnotation,
		[]string{"aScheduleID"},
	)

	var conf incidentio.Config
	conf.Teleport = s.TeleportConfig()
	conf.ClientConfig.APIEndpoint = s.fakeIncident.URL()
	conf.ClientConfig.AlertSourceEndpoint = s.fakeIncident.URL() + "/v2/alert_events/http/someRequestID"
	conf.PluginType = types.PluginTypeIncidentio

	s.appConfig = conf
}

// startApp starts the incident.io plugin, waits for it to become ready and returns.
func (s *IncidentBaseSuite) startApp() {
	s.T().Helper()
	t := s.T()

	app, err := incidentio.NewIncidentApp(context.Background(), &s.appConfig)
	require.NoError(t, err)
	integration.RunAndWaitReady(t, app)
}

// IncidentSuiteOSS contains all tests that support running against a Teleport
// OSS Server.
type IncidentSuiteOSS struct {
	IncidentBaseSuite
}

// IncidentSuiteEnterprise contains all tests that require a Teleport Enterprise
// to run.
type IncidentSuiteEnterprise struct {
	IncidentBaseSuite
}

// SetupTest overrides IncidentBaseSuite.SetupTest to check the Teleport features
// before each test.
func (s *IncidentSuiteEnterprise) SetupTest() {
	t := s.T()
	s.RequireAdvancedWorkflow(t)
	s.IncidentBaseSuite.SetupTest()
}

// TestAlertCreationForSchedules validates that an alert is created to the service
// specified in the role's annotation using /notify-services annotation
func (s *IncidentSuiteOSS) TestAlertCreationForSchedules() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test execution: create an access request
	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)

	// Validate the alert has been created in incident.io and its ID is stored in
	// the plugin_data.
	pluginData := s.checkPluginData(ctx, req.GetName(), func(data incidentio.PluginData) bool {
		return data.DeduplicationKey != ""
	})

	alert, err := s.fakeIncident.CheckNewAlert(ctx)

	require.NoError(t, err, "no new alerts stored")

	assert.Equal(t, alert.DeduplicationKey, pluginData.DeduplicationKey)
}

// TestApproval tests that when a request is approved, its corresponding alert
// is updated to reflect the new request state and a note is added to the alert.
func (s *IncidentSuiteOSS) TestApproval() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its alert.
	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)

	_, err := s.fakeIncident.CheckNewAlert(ctx)
	require.NoError(t, err, "no new alerts stored")

	// Test execution: we approve the request
	err = s.Ruler().ApproveAccessRequest(ctx, req.GetName(), "okay")
	require.NoError(t, err)

	// Validating the plugin resolved the alert.
	alertUpdate, err := s.fakeIncident.CheckAlertUpdate(ctx)
	require.NoError(t, err)
	assert.Equal(t, "resolved", alertUpdate.Status)
}

// TestDenial tests that when a request is denied, its corresponding alert
// is updated to reflect the new request state and a note is added to the alert.
func (s *IncidentSuiteOSS) TestDenial() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its alert.
	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)

	_, err := s.fakeIncident.CheckNewAlert(ctx)
	require.NoError(t, err, "no new alerts stored")

	// Test execution: we deny the request
	err = s.Ruler().DenyAccessRequest(ctx, req.GetName(), "not okay")
	require.NoError(t, err)

	// Validating the plugin resolved the alert.
	alertUpdate, err := s.fakeIncident.CheckAlertUpdate(ctx)
	require.NoError(t, err)
	assert.Equal(t, "resolved", alertUpdate.Status)
}

// TestApprovalByReview tests that the alert is annotated and resolved after the
// access request approval threshold is reached.
func (s *IncidentSuiteEnterprise) TestApprovalByReview() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its alert.
	req := s.CreateAccessRequest(ctx, integration.Requester1UserName, nil)

	_, err := s.fakeIncident.CheckNewAlert(ctx)
	require.NoError(t, err, "no new alerts stored")

	// Test execution: we submit two reviews
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

	// Validate the alert got resolved.
	data := s.checkPluginData(ctx, req.GetName(), func(data incidentio.PluginData) bool {
		return data.ReviewsCount == 2 && data.Resolution.Tag != incidentio.Unresolved
	})
	assert.Equal(t, incidentio.Resolution{Tag: incidentio.ResolvedApproved, Reason: "finally okay"}, data.Resolution)

	alertUpdate, err := s.fakeIncident.CheckAlertUpdate(ctx)
	require.NoError(t, err)
	assert.Equal(t, "resolved", alertUpdate.Status)
}

// TestDenialByReview tests that the alert is annotated and resolved after the
// access request denial threshold is reached.
func (s *IncidentSuiteEnterprise) TestDenialByReview() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its alert.
	req := s.CreateAccessRequest(ctx, integration.Requester1UserName, nil)

	_, err := s.fakeIncident.CheckNewAlert(ctx)
	require.NoError(t, err, "no new alerts stored")

	// Test execution: we submit two reviews
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

	// Validate the alert got resolved.
	data := s.checkPluginData(ctx, req.GetName(), func(data incidentio.PluginData) bool {
		return data.ReviewsCount == 2 && data.Resolution.Tag != incidentio.Unresolved
	})
	assert.Equal(t, incidentio.Resolution{Tag: incidentio.ResolvedDenied, Reason: "finally not okay"}, data.Resolution)

	alertUpdate, err := s.fakeIncident.CheckAlertUpdate(ctx)
	require.NoError(t, err)
	assert.Equal(t, "resolved", alertUpdate.Status)
}

// TestAutoApprovalWhenNotOnCall tests that access requests are not automatically
// approved when the user is not on-call.
func (s *IncidentSuiteEnterprise) TestAutoApprovalWhenNotOnCall() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// Test setup: create an on-call schedule with a non-Teleport user in it.
	s.fakeIncident.StoreSchedule(ApprovalScheduleName, s.incSchedule)
	s.AnnotateRequesterRoleAccessRequests(
		ctx,
		ApprovalScheduleAnnotation,
		[]string{ApprovalScheduleName},
	)

	s.startApp()

	// Test Execution: we create an access request and wait for its alert.
	req := s.CreateAccessRequest(ctx, integration.Requester1UserName, nil)

	_ = s.checkPluginData(ctx, req.GetName(), func(data incidentio.PluginData) bool {
		return data.DeduplicationKey != ""
	})

	_, err := s.fakeIncident.CheckNewAlert(ctx)
	require.NoError(t, err, "no new alerts stored")

	// Fetch updated access request
	req, err = s.Ruler().GetAccessRequest(ctx, req.GetName())
	require.NoError(t, err)

	require.Empty(t, req.GetReviews(), "no review should be submitted automatically")
}

// TestAutoApprovalWhenOnCall tests that access requests are automatically
// approved when the user is not on-call.
func (s *IncidentSuiteEnterprise) TestAutoApprovalWhenOnCall() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// Test setup: create an on-call schedule with a non-Teleport user in it.
	s.fakeIncident.StoreSchedule(ApprovalScheduleName, s.incSchedule)
	s.AnnotateRequesterRoleAccessRequests(
		ctx,
		ApprovalScheduleAnnotation,
		[]string{ApprovalScheduleName},
	)

	s.startApp()

	// Test Execution: we create an access request and wait for its alert.
	req := s.CreateAccessRequest(ctx, integration.Requester1UserName, nil)

	_ = s.checkPluginData(ctx, req.GetName(), func(data incidentio.PluginData) bool {
		return data.DeduplicationKey != ""
	})

	_, err := s.fakeIncident.CheckNewAlert(ctx)
	require.NoError(t, err, "no new alerts stored")

	// Fetch updated access request
	req, err = s.Ruler().GetAccessRequest(ctx, req.GetName())
	require.NoError(t, err)

	reviews := req.GetReviews()
	require.Len(t, reviews, 1, "a review should be submitted automatically")
	require.Equal(t, types.RequestState_APPROVED, reviews[0].ProposedState)
}
