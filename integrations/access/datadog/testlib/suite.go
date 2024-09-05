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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/accessrequest"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/datadog"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/plugindata"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
)

type DatadogBaseSuite struct {
	*integration.AccessRequestSuite
	appConfig   *datadog.Config
	fakeDatadog *FakeDatadog

	reviewer1 string
	team1     string
}

func (s *DatadogBaseSuite) SetupTest() {
	t := s.T()

	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)

	s.fakeDatadog = NewFakeDatadog()
	t.Cleanup(s.fakeDatadog.Close)

	s.reviewer1 = "reviewer1"
	s.team1 = "team1"

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

func (s *DatadogBaseSuite) startApp() {
	t := s.T()
	t.Helper()

	app := datadog.NewDatadogApp(s.appConfig)
	integration.RunAndWaitReady(t, app)
}

type DatadogSuiteOSS struct {
	DatadogBaseSuite
}

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

	incident, err := s.fakeDatadog.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")

	assert.Equal(t, incident.Data.ID, pluginData.SentMessages[0].MessageID)
	assert.Equal(t, fmt.Sprintf("@%s", integration.Reviewer1UserName), incident.Data.Attributes.NotificationHandles[0].Handle)
	assert.Equal(t, "active", incident.Data.Attributes.Fields.State.Value)
}

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
	assert.Contains(t, note.Data.Attributes.Content.Content, "Access request is ✅ APPROVED")
	assert.Contains(t, note.Data.Attributes.Content.Content, "Reason: okay")

	// Validating the plugin resolved the incident.
	incidentUpdate, err := s.fakeDatadog.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Equal(t, "resolved", incidentUpdate.Data.Attributes.Fields.State.Value)
}

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
	assert.Contains(t, note.Data.Attributes.Content.Content, "Access request is ❌ DENIED")
	assert.Contains(t, note.Data.Attributes.Content.Content, "Reason: not okay")

	// Validating the plugin resolved the incident.
	incidentUpdate, err := s.fakeDatadog.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Equal(t, "resolved", incidentUpdate.Data.Attributes.Fields.State.Value)
}

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

	// Validate incident notes were sent with the correct content.
	pluginData := s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0 && data.ReviewsCount == 2
	})

	note, err := s.fakeDatadog.CheckNewIncidentNote(ctx)
	require.NoError(t, err)

	content := note.Data.Attributes.Content.Content
	assert.Equal(t, pluginData.SentMessages[0].MessageID, note.Data.ID)
	assert.Contains(t, content, integration.Reviewer1UserName+" reviewed the request", "note must contain a review author")
	assert.Contains(t, content, "Resolution: APPROVED", "note must contain an approval resolution")
	assert.Contains(t, content, "Reason: okay", "note must contain an approval reason")

	note, err = s.fakeDatadog.CheckNewIncidentNote(ctx)
	require.NoError(t, err)

	content = note.Data.Attributes.Content.Content
	assert.Equal(t, pluginData.SentMessages[0].MessageID, note.Data.ID)
	assert.Contains(t, content, integration.Reviewer2UserName+" reviewed the request", "note must contain a review author")
	assert.Contains(t, content, "Resolution: DENIED", "note must contain a denial resolution")
	assert.Contains(t, content, "Reason: not okay", "note must contain a denial reason")
}

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
	data := s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return data.ReviewsCount == 2 && data.ResolutionTag != plugindata.Unresolved
	})
	assert.Equal(t, plugindata.ResolvedApproved, data.ResolutionTag)
	assert.Equal(t, "finally okay", data.ResolutionReason)

	note, err = s.fakeDatadog.CheckNewIncidentNote(ctx)
	require.NoError(t, err)

	content = note.Data.Attributes.Content.Content
	require.Contains(t, content, "Access request has been approved")
	require.Contains(t, content, "Reason: finally okay")

	incidentUpdate, err := s.fakeDatadog.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Equal(t, "resolved", incidentUpdate.Data.Attributes.Fields.State.Value)
}

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
	data := s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return data.ReviewsCount == 2 && data.ResolutionTag != plugindata.Unresolved
	})
	assert.Equal(t, plugindata.ResolvedDenied, data.ResolutionTag)
	assert.Equal(t, "finally not okay", data.ResolutionReason)

	note, err = s.fakeDatadog.CheckNewIncidentNote(ctx)
	require.NoError(t, err)

	content = note.Data.Attributes.Content.Content
	assert.Contains(t, content, "Access request has been denied")
	assert.Contains(t, content, "Reason: finally not okay")

	incidentUpdate, err := s.fakeDatadog.CheckIncidentUpdate(ctx)
	require.NoError(t, err)
	assert.Equal(t, "resolved", incidentUpdate.Data.Attributes.Fields.State.Value)
}

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
	request := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)

	pluginData := s.checkPluginData(ctx, request.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})

	incident, err := s.fakeDatadog.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")

	assert.Equal(t, incident.Data.ID, pluginData.SentMessages[0].MessageID)
	assert.Equal(t, fmt.Sprintf("@%s", integration.Reviewer1UserName), incident.Data.Attributes.NotificationHandles[0].Handle)
	assert.Equal(t, "active", incident.Data.Attributes.Fields.State.Value)
	assert.NoError(t, s.ClientByName(integration.RulerUserName).
		AccessMonitoringRulesClient().DeleteAccessMonitoringRule(ctx, "test-datadog-amr"))
}

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
	request := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)
	pluginData := s.checkPluginData(ctx, request.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})

	incident, err := s.fakeDatadog.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")

	assert.Equal(t, incident.Data.ID, pluginData.SentMessages[0].MessageID)
	assert.Equal(t, fmt.Sprintf("@%s", integration.Reviewer1UserName), incident.Data.Attributes.NotificationHandles[0].Handle)
	assert.Equal(t, "active", incident.Data.Attributes.Fields.State.Value)

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
	request = s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)
	pluginData = s.checkPluginData(ctx, request.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})

	incident, err = s.fakeDatadog.CheckNewIncident(ctx)
	require.NoError(t, err, "no new incidents stored")

	assert.Equal(t, incident.Data.ID, pluginData.SentMessages[0].MessageID)
	assert.Equal(t, fmt.Sprintf("@%s", integration.Reviewer2UserName), incident.Data.Attributes.NotificationHandles[0].Handle)
	assert.Equal(t, "active", incident.Data.Attributes.Fields.State.Value)

	assert.NoError(t, s.ClientByName(integration.RulerUserName).
		AccessMonitoringRulesClient().DeleteAccessMonitoringRule(ctx, "test-datadog-amr-2"))
}
