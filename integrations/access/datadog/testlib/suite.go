/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/accessrequest"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/datadog"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/plugindata"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
	"github.com/gravitational/trace"
)

// DatadogBaseSuite is the Datadog Incident Management plugin test suite.
// It implements the testify.TestingSuite interface.
type DatadogBaseSuite struct {
	*integration.AccessRequestSuite
	appConfig   *datadog.Config
	fakeDatadog *FakeDatadog

	raceNumber int
}

// SetupTest starts a fake Datadog service and geneates the plugin configuration.
// It runs for each test.
func (s *DatadogBaseSuite) SetupTest() {
	t := s.T()

	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)

	s.raceNumber = runtime.GOMAXPROCS(0)
	s.fakeDatadog = NewFakeDatadog()
	t.Cleanup(s.fakeDatadog.Close)

	s.appConfig = &datadog.Config{
		BaseConfig: common.BaseConfig{
			Teleport:   s.TeleportConfig(),
			PluginType: types.PluginTypeDatadog,
		},
		Datadog: datadog.DatadogConfig{
			APIEndpoint:    s.fakeDatadog.URL() + "/",
			APIKey:         "api-key",
			ApplicationKey: "application-key",
		},
		StatusSink: &integration.FakeStatusSink{},
	}
}

// startApp starts the Datadog Incident Management plugin, waits for it to become ready and returns.
func (s *DatadogBaseSuite) startApp() {
	t := s.T()
	t.Helper()

	app := datadog.NewDatadogApp(s.appConfig)
	integration.RunAndWaitReady(t, app)
}

// DatadogSuiteOSS contains all tests that support running against a Teleport
// OSS Server.
type DatadogSuiteOSS struct {
	DatadogBaseSuite
}

// DatadogSuiteEnterprise contains all tests that require a Teleport Enterprise
// to run.
type DatadogSuiteEnterprise struct {
	DatadogBaseSuite
}

// SetupTest overrides DatadogBaseSuite.SetupTest to check the Teleport features
// before each test.
func (s *DatadogSuiteEnterprise) SetupTest() {
	t := s.T()
	s.RequireAdvancedWorkflow(t)
	s.DatadogBaseSuite.SetupTest()
}

// TestIncidentCreation validates that an active incident is created and the
// suggested reviewers are notified.
func (s *DatadogSuiteOSS) TestIncidentCreation() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its incident.
	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, []string{
		integration.Reviewer1UserName,
	})

	pluginData := s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})
	require.Equal(t, len(pluginData.SentMessages), 1)

	incident, err := s.fakeDatadog.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")
	require.Equal(t, len(incident.Data.Attributes.NotificationHandles), 1)

	assert.Equal(t, incident.Data.ID, pluginData.SentMessages[0].MessageID)
	assert.Equal(t, fmt.Sprintf("@%s", integration.Reviewer1UserName), incident.Data.Attributes.NotificationHandles[0].Handle)
	assert.Equal(t, "active", incident.Data.Attributes.Fields.State.Value)
}

// TestApproval tests that when a request is approved, its corresponding incident
// is updated to reflect the new request state.
func (s *DatadogSuiteOSS) TestApproval() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its incident.
	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, []string{
		integration.Reviewer1UserName,
	})

	_, err := s.fakeDatadog.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")

	// Test execution: we approve the request
	err = s.Ruler().ApproveAccessRequest(ctx, req.GetName(), "okay")
	require.NoError(t, err)

	// Validating the plugin added a note to the incident explaining it got approved.
	note, err := s.fakeDatadog.CheckNewIncidentNote(ctx)
	require.NoError(t, err)

	content := note.Data.Attributes.Content.Content
	assert.Contains(t, content, "Access request is ✅ APPROVED")
	assert.Contains(t, content, "Reason: okay")

	// Validating the plugin resolved the incident.
	incidentUpdate, err := s.fakeDatadog.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Equal(t, "resolved", incidentUpdate.Data.Attributes.Fields.State.Value)
}

// TestDenial tests that when a request is denied, its corresponding incident
// is updated to reflect the new request state.
func (s *DatadogSuiteOSS) TestDenial() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its incident.
	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, []string{
		integration.Reviewer1UserName,
	})

	_, err := s.fakeDatadog.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")

	// Test execution: we approve the request
	err = s.Ruler().DenyAccessRequest(ctx, req.GetName(), "not okay")
	require.NoError(t, err)

	// Validating the plugin added a note to the incident explaining it got denied.
	note, err := s.fakeDatadog.CheckNewIncidentNote(ctx)
	require.NoError(t, err)

	content := note.Data.Attributes.Content.Content
	assert.Contains(t, content, "Access request is ❌ DENIED")
	assert.Contains(t, content, "Reason: not okay")

	// Validating the plugin resolved the incident.
	incidentUpdate, err := s.fakeDatadog.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Equal(t, "resolved", incidentUpdate.Data.Attributes.Fields.State.Value)
}

// TestExpiration tests that when a request expires, its corresponding incident
// is updated to reflect the new request state.
func (s *DatadogSuiteOSS) TestExpiration() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its incident.
	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, []string{
		integration.Reviewer1UserName,
	})

	incident, err := s.fakeDatadog.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")
	assert.Equal(t, "active", incident.Data.Attributes.Fields.State.Value)
	incidentID := incident.Data.ID

	s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})

	// Test execution: we expire the request
	err = s.Ruler().DeleteAccessRequest(ctx, req.GetName()) // simulate expiration
	require.NoError(t, err)

	// Validating the plugin resolved the incident and added a note explaining the reason.
	incident, err = s.fakeDatadog.CheckIncidentUpdate(ctx)
	require.NoError(t, err, "no new incidents updated")
	assert.Equal(t, incidentID, incident.Data.ID)
	assert.Equal(t, "resolved", incident.Data.Attributes.Fields.State.Value)
}

// TestRecipientsFromAccessMonitoringRule tests access monitoring rules are
// applied to the recipient selection process.
func (s *DatadogSuiteOSS) TestRecipientsFromAccessMonitoringRule() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// Setup base config to ensure access monitoring rule recipient take precidence
	s.appConfig.Recipients = common.RawRecipientsMap{
		types.Wildcard: []string{
			integration.Reviewer2UserName,
		},
	}

	s.startApp()

	_, err := s.ClientByName(integration.RulerUserName).
		AccessMonitoringRulesClient().
		CreateAccessMonitoringRule(ctx, &accessmonitoringrulesv1.AccessMonitoringRule{
			Kind:    types.KindAccessMonitoringRule,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "test-datadog-amr",
			},
			Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
				Subjects:  []string{types.KindAccessRequest},
				Condition: "!is_empty(access_request.spec.roles)",
				Notification: &accessmonitoringrulesv1.Notification{
					Name: "datadog",
					Recipients: []string{
						integration.Reviewer1UserName,
					},
				},
			},
		})
	require.NoError(t, err)

	// Test execution: create an access request
	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)

	pluginData := s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})
	require.Len(t, pluginData.SentMessages, 1)

	incident, err := s.fakeDatadog.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")

	assert.Equal(t, incident.Data.ID, pluginData.SentMessages[0].MessageID)
	assert.Equal(t, fmt.Sprintf("@%s", integration.Reviewer1UserName), incident.Data.Attributes.NotificationHandles[0].Handle)
	assert.Equal(t, "active", incident.Data.Attributes.Fields.State.Value)
	assert.NoError(t, s.ClientByName(integration.RulerUserName).
		AccessMonitoringRulesClient().DeleteAccessMonitoringRule(ctx, "test-datadog-amr"))
}

// TestRecipientsFromAccessMonitoringRuleAfterUpdate tests access monitoring
// rules are respected after an the rule is updated.
func (s *DatadogSuiteOSS) TestRecipientsFromAccessMonitoringRuleAfterUpdate() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// Setup base config to ensure access monitoring rule recipient take precidence
	s.appConfig.Recipients = common.RawRecipientsMap{
		types.Wildcard: []string{
			integration.Reviewer2UserName,
		},
	}

	s.startApp()

	_, err := s.ClientByName(integration.RulerUserName).
		AccessMonitoringRulesClient().
		CreateAccessMonitoringRule(ctx, &accessmonitoringrulesv1.AccessMonitoringRule{
			Kind:    types.KindAccessMonitoringRule,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "test-datadog-amr-2",
			},
			Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
				Subjects:  []string{types.KindAccessRequest},
				Condition: "!is_empty(access_request.spec.roles)",
				Notification: &accessmonitoringrulesv1.Notification{
					Name: "datadog",
					Recipients: []string{
						integration.Reviewer1UserName,
					},
				},
			},
		})
	assert.NoError(t, err)

	// Test execution: we create an access request
	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)
	pluginData := s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})
	require.Len(t, pluginData.SentMessages, 1)

	incident, err := s.fakeDatadog.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")

	assert.Equal(t, incident.Data.ID, pluginData.SentMessages[0].MessageID)
	assert.Equal(t, fmt.Sprintf("@%s", integration.Reviewer1UserName), incident.Data.Attributes.NotificationHandles[0].Handle)
	assert.Equal(t, "active", incident.Data.Attributes.Fields.State.Value)

	// Update the Access Monitoring Rule so it is no longer applied
	_, err = s.ClientByName(integration.RulerUserName).
		AccessMonitoringRulesClient().
		UpdateAccessMonitoringRule(ctx, &accessmonitoringrulesv1.AccessMonitoringRule{
			Kind:    types.KindAccessMonitoringRule,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "test-datadog-amr-2",
			},
			Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
				Subjects:  []string{"someOtherKind"},
				Condition: "!is_empty(access_request.spec.roles)",
				Notification: &accessmonitoringrulesv1.Notification{
					Name: "datadog",
					Recipients: []string{
						integration.Reviewer1UserName,
					},
				},
			},
		})
	assert.NoError(t, err)

	// Test execution: we create an access request
	req = s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)
	pluginData = s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})
	require.Len(t, pluginData.SentMessages, 1)

	incident, err = s.fakeDatadog.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")

	assert.Equal(t, incident.Data.ID, pluginData.SentMessages[0].MessageID)
	assert.Equal(t, fmt.Sprintf("@%s", integration.Reviewer2UserName), incident.Data.Attributes.NotificationHandles[0].Handle)
	assert.Equal(t, "active", incident.Data.Attributes.Fields.State.Value)

	assert.NoError(t, s.ClientByName(integration.RulerUserName).
		AccessMonitoringRulesClient().DeleteAccessMonitoringRule(ctx, "test-datadog-amr-2"))
}

// TestReviewNotes tests that a new note is added to the incident after the
// access request is reviewed.
func (s *DatadogSuiteEnterprise) TestReviewNotes() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request.
	req := s.CreateAccessRequest(ctx, integration.Requester1UserName, []string{
		integration.Reviewer1UserName,
		integration.Reviewer2UserName,
	})

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

	// Validate incident notes were created with the correct content.
	pluginData := s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0 && data.ReviewsCount == 2
	})
	assert.Equal(t, len(pluginData.SentMessages), 1)

	note, err := s.fakeDatadog.CheckNewIncidentNote(ctx)
	require.NoError(t, err)

	content := note.Data.Attributes.Content.Content
	assert.Contains(t, content, integration.Reviewer1UserName+" reviewed the request", "note must contain a review author")
	assert.Contains(t, content, "Resolution: APPROVED", "note must contain an approval resolution")
	assert.Contains(t, content, "Reason: okay", "note must contain an approval reason")

	note, err = s.fakeDatadog.CheckNewIncidentNote(ctx)
	require.NoError(t, err)

	content = note.Data.Attributes.Content.Content
	assert.Contains(t, content, integration.Reviewer2UserName+" reviewed the request", "note must contain a review author")
	assert.Contains(t, content, "Resolution: DENIED", "note must contain a denial resolution")
	assert.Contains(t, content, "Reason: not okay", "note must contain a denial reason")
}

// TestApprovalByReview tests that the incident is updated after the access
// request is reviewed and approved.
func (s *DatadogSuiteEnterprise) TestApprovalByReview() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its incident.
	req := s.CreateAccessRequest(ctx, integration.Requester1UserName, []string{
		integration.Reviewer1UserName,
		integration.Reviewer2UserName,
	})

	_, err := s.fakeDatadog.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")

	// Test execution: we submit a review and validate that a note was created.
	err = s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	note, err := s.fakeDatadog.CheckNewIncidentNote(ctx)
	require.NoError(t, err)

	content := note.Data.Attributes.Content.Content
	assert.Contains(t, content, integration.Reviewer1UserName+" reviewed the request", "note must contain a review author")

	// Test execution: we submit a second review and validate that a note was created.
	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "finally okay",
	})
	require.NoError(t, err)

	note, err = s.fakeDatadog.CheckNewIncidentNote(ctx)
	require.NoError(t, err)

	content = note.Data.Attributes.Content.Content
	assert.Contains(t, content, integration.Reviewer2UserName+" reviewed the request", "note must contain a review author")

	// Validate the alert got resolved, and a final note was added describing the resolution.
	pluginData := s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return data.ReviewsCount == 2 && data.ResolutionTag != plugindata.Unresolved
	})
	assert.Equal(t, plugindata.ResolvedApproved, pluginData.ResolutionTag)
	assert.Equal(t, "finally okay", pluginData.ResolutionReason)

	note, err = s.fakeDatadog.CheckNewIncidentNote(ctx)
	require.NoError(t, err)

	content = note.Data.Attributes.Content.Content
	require.Contains(t, content, "Access request has been approved")
	require.Contains(t, content, "Reason: finally okay")

	incidentUpdate, err := s.fakeDatadog.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Equal(t, "resolved", incidentUpdate.Data.Attributes.Fields.State.Value)
}

// TestDenialByReview tests that the incident is updated after the access request
// is reviewed and denied.
func (s *DatadogSuiteEnterprise) TestDenialByReview() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its incident.
	req := s.CreateAccessRequest(ctx, integration.Requester1UserName, []string{
		integration.Reviewer1UserName,
		integration.Reviewer2UserName,
	})

	_, err := s.fakeDatadog.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")

	// Test execution: we submit a review and validate that a note was created.
	err = s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	note, err := s.fakeDatadog.CheckNewIncidentNote(ctx)
	require.NoError(t, err)

	content := note.Data.Attributes.Content.Content
	assert.Contains(t, content, integration.Reviewer1UserName+" reviewed the request", "note must contain a review author")

	// Test execution: we submit a review and validate that a note was created.
	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "finally not okay",
	})
	require.NoError(t, err)

	note, err = s.fakeDatadog.CheckNewIncidentNote(ctx)
	require.NoError(t, err)

	content = note.Data.Attributes.Content.Content
	assert.Contains(t, content, integration.Reviewer2UserName+" reviewed the request", "note must contain a review author")

	// Validate the alert got resolved, and a final note was added describing the resolution.
	pluginData := s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return data.ReviewsCount == 2 && data.ResolutionTag != plugindata.Unresolved
	})
	assert.Equal(t, plugindata.ResolvedDenied, pluginData.ResolutionTag)
	assert.Equal(t, "finally not okay", pluginData.ResolutionReason)

	note, err = s.fakeDatadog.CheckNewIncidentNote(ctx)
	require.NoError(t, err)

	content = note.Data.Attributes.Content.Content
	assert.Contains(t, content, "Access request has been denied")
	assert.Contains(t, content, "Reason: finally not okay")

	incidentUpdate, err := s.fakeDatadog.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Equal(t, "resolved", incidentUpdate.Data.Attributes.Fields.State.Value)
}

// TestRace validates that the plugin behaves properly and performs all the
// incident updates when a lot of access requests are sent and reviewed in a
// very short time frame.
func (s *DatadogSuiteEnterprise) TestRace() {
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

	watcher, err := s.Ruler().NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{{Kind: types.KindAccessRequest}},
	})
	require.NoError(t, err)
	defer watcher.Close()
	assert.Equal(t, types.OpInit, (<-watcher.Events()).Type)
	assert.Equal(t, 0, s.raceNumber)

	process := lib.NewProcess(ctx)
	for i := 0; i < 2*s.raceNumber; i++ {
		var requester string
		var proposedState types.RequestState
		reviewsNumber := 2
		switch i % 2 {
		case 0:
			requester = integration.Requester1UserName
			proposedState = types.RequestState_APPROVED
		case 1:
			requester = integration.Requester2UserName
			proposedState = types.RequestState_DENIED
		}

		// Create access requests
		process.SpawnCritical(func(ctx context.Context) error {
			req, err := types.NewAccessRequest(uuid.New().String(), requester, "editor")
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			req.SetSuggestedReviewers([]string{integration.Reviewer1UserName, integration.Reviewer2UserName})
			if _, err := s.Requester1().CreateAccessRequestV2(ctx, req); err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			return nil
		})

		// Verify incident creation and review access requests
		process.SpawnCritical(func(ctx context.Context) error {
			incident, err := s.fakeDatadog.CheckNewIncident(ctx)
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			if obtained, expected := incident.Data.Attributes.Fields.State.Value, "active"; obtained != expected {
				return setRaceErr(trace.Errorf("wrong incident status. expected %s, obtained %s", expected, obtained))
			}

			if _, loaded := incidentIDs.LoadOrStore(incident.Data.ID, struct{}{}); loaded {
				return setRaceErr(trace.Errorf("incident %s has already been stored", incident.Data.ID))
			}

			atomic.AddInt32(&incidentsCount, 1)

			reqID, err := parseSummaryField(incident.Data.Attributes.Fields.Summary.Value, "ID")
			if err != nil {
				return setRaceErr(trace.Wrap(err))
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

		// Verify incidents are resolved
		process.SpawnCritical(func(ctx context.Context) error {
			incident, err := s.fakeDatadog.CheckIncidentUpdate(ctx)
			if err := trace.Wrap(err); err != nil {
				return setRaceErr(err)
			}
			if obtained, expected := incident.Data.Attributes.Fields.State.Value, "resolved"; obtained != expected {
				return setRaceErr(trace.Errorf("wrong incident status. expected %s, obtained %s", expected, obtained))
			}
			return nil
		})
	}

	// Count the number of notes created
	for i := 0; i < 3*s.raceNumber; i++ {
		process.SpawnCritical(func(ctx context.Context) error {
			_, err := s.fakeDatadog.CheckNewIncidentNote(ctx)
			if err := trace.Wrap(err); err != nil {
				return setRaceErr(err)
			}

			var newCounter int32
			val, _ := incidentNoteCounters.LoadOrStore("incident_note_count", &newCounter)
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

	val, ok := incidentNoteCounters.LoadAndDelete("incident_note_count")
	require.True(t, ok)

	counterPtr := val.(*int32)
	assert.Equal(t, int32(3*s.raceNumber), *counterPtr)
	assert.Equal(t, int32(s.raceNumber), incidentsCount)
}
