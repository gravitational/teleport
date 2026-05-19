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
	"net/http"
	"net/url"
	"regexp"
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
	"github.com/gravitational/teleport/integrations/access/jira"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
)

// JiraBaseSuite is the Jira access plugin test suite.
// It implements the testify.TestingSuite interface.
type JiraBaseSuite struct {
	*integration.AccessRequestSuite
	appConfig  jira.Config
	raceNumber int

	// jira user that changes the issue state and comments the approval/denial reason
	// this user doesn't have to match any Teleport user.
	jiraReviewerUser jira.UserDetails
	// jira user that comments in the jira issue but is not the one who approved/denied the request
	jiraOtherUser jira.UserDetails

	fakeJira   *FakeJira
	webhookURL *url.URL
}

// SetupTest starts a fake discord and generates the plugin configuration.
// It is run for each test.
func (s *JiraBaseSuite) SetupTest() {
	t := s.T()

	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)
	s.raceNumber = runtime.GOMAXPROCS(0)

	s.jiraReviewerUser = jira.UserDetails{AccountID: "JIRA-REVIEWER", DisplayName: "Jira Reviewer 1", EmailAddress: "jira-reviewer@example.com"}
	s.jiraOtherUser = jira.UserDetails{AccountID: "RANDOM-USER", DisplayName: "Reviewer 1 evil twin", EmailAddress: "randomfolk@example.com"}

	pluginUser := jira.UserDetails{AccountID: "PLUGIN", DisplayName: "Teleport access plugin", EmailAddress: integration.PluginUserName}
	s.fakeJira = NewFakeJira(pluginUser, s.raceNumber)
	t.Cleanup(s.fakeJira.Close)

	var conf jira.Config
	conf.Teleport = s.TeleportConfig()
	conf.Jira.URL = s.fakeJira.URL()
	conf.Jira.Username = integration.PluginUserName
	conf.Jira.APIToken = "xyz"
	conf.Jira.Project = "PROJ"
	conf.HTTP.ListenAddr = "127.0.0.1:0"
	conf.HTTP.Insecure = true
	conf.DisableWebhook = false

	s.appConfig = conf
}

// startApp starts the discord plugin, waits for it to become ready and returns,
func (s *JiraBaseSuite) startApp() {
	s.T().Helper()
	t := s.T()

	app, err := jira.NewApp(s.appConfig)
	require.NoError(t, err)
	integration.RunAndWaitReady(t, app)
	s.webhookURL = app.PublicURL()
}

// JiraSuiteOSS contains all tests that support running against a Teleport
// OSS Server.
type JiraSuiteOSS struct {
	JiraBaseSuite
}

// JiraSuiteEnterprise contains all tests that require a Teleport Enterprise
// to run.
type JiraSuiteEnterprise struct {
	JiraBaseSuite
}

// SetupTest overrides JiraBaseSuite.SetupTest to check the Teleport features
// before each test.
func (s *JiraSuiteEnterprise) SetupTest() {
	t := s.T()
	s.RequireAdvancedWorkflow(t)
	s.JiraBaseSuite.SetupTest()
}

// TestIssueCreation validates that an issue is created when a new access
// request is created. It also checks that the issue content is correct.
func (s *JiraSuiteOSS) TestIssueCreation() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test execution: creating a new access request
	request := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)

	pluginData := s.checkPluginData(ctx, request.GetName(), func(data jira.PluginData) bool {
		return data.IssueID != ""
	}) // when issue id is written, we are sure that request is completely served.

	issue, err := s.fakeJira.CheckNewIssue(ctx)
	require.NoError(t, err, "no new issue stored")
	assert.Equal(t, "PROJ", issue.Fields.Project.Key)
	assert.Equal(t, request.GetName(), issue.Properties[jira.RequestIDPropertyKey])
	assert.Equal(t, pluginData.IssueID, issue.ID)
}

// TestIssueCreationWithRequestReason validates that an issue is created when
// a new access request is created. It also checks that the issue content
// reflects the access request reason.
func (s *JiraSuiteOSS) TestIssueCreationWithRequestReason() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test execution: creating a new access request
	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)
	s.checkPluginData(ctx, req.GetName(), func(data jira.PluginData) bool {
		return data.IssueID != ""
	}) // when issue id is written, we are sure that request is completely served.

	issue, err := s.fakeJira.CheckNewIssue(ctx)
	require.NoError(t, err)

	if !strings.Contains(issue.Fields.Description, `Reason: *because of*`) {
		t.Error("Issue description should contain request reason")
	}
}

// TestIssueCreationWithLargeRequestReason validates that an issue is created
// when a new access request with a large reason is created.
func (s *JiraSuiteOSS) TestIssueCreationWithLargeRequestReason() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test execution: creating a new access request with a very large reason
	s.SetReasonPadding(jira.JiraReasonLimit + 10)
	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)
	s.checkPluginData(ctx, req.GetName(), func(data jira.PluginData) bool {
		return data.IssueID != ""
	}) // when issue id is written, we are sure that request is completely served.

	issue, err := s.fakeJira.CheckNewIssue(ctx)
	require.NoError(t, err)
	re := regexp.MustCompile("Reason...because of (A+)")
	match := re.FindStringSubmatch(issue.Fields.Description)
	if len(match) != 2 {
		t.Error("reason not found in issue description")
		return
	}

	// Validate the reason got truncated to the max length
	require.Len(t, match[1], jira.JiraReasonLimit-len("because of "))
	s.SetReasonPadding(0)
}

// TestCustomIssueType tests that requests can use a custom issue type.
func (s *JiraSuiteOSS) TestCustomIssueType() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.appConfig.Jira.IssueType = "Story"
	s.startApp()

	// Test setup: we create an access request
	_ = s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)

	// We validate that the issue was created using the Issue Type "Story"
	newIssue, err := s.fakeJira.CheckNewIssue(ctx)
	require.NoError(t, err)
	require.Equal(t, "Story", newIssue.Fields.Type.Name)
}

// TestReviewComments tests that comments are posted for each access request
// review.
func (s *JiraSuiteEnterprise) TestReviewComments() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request
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

	// We validate the plugin saw the two reviews and put them in the plugin_data.
	pluginData := s.checkPluginData(ctx, req.GetName(), func(data jira.PluginData) bool {
		return data.IssueID != "" && data.ReviewsCount == 2
	})

	// We validate that 2 comments were posted: one for each review.
	comment, err := s.fakeJira.CheckNewIssueComment(ctx)
	require.NoError(t, err)
	assert.Equal(t, pluginData.IssueID, comment.IssueID)
	assert.Contains(t, comment.Body, "*"+integration.Reviewer1UserName+"* reviewed the request", "comment must contain a review pluginUser")
	assert.Contains(t, comment.Body, "Resolution: *APPROVED*", "comment must contain an approval resolution")
	assert.Contains(t, comment.Body, "Reason: okay", "comment must contain an approval reason")

	comment, err = s.fakeJira.CheckNewIssueComment(ctx)
	require.NoError(t, err)
	assert.Equal(t, pluginData.IssueID, comment.IssueID)
	assert.Contains(t, comment.Body, "*"+integration.Reviewer2UserName+"* reviewed the request", "comment must contain a review pluginUser")
	assert.Contains(t, comment.Body, "Resolution: *DENIED*", "comment must contain a denial resolution")
	assert.Contains(t, comment.Body, "Reason: not okay", "comment must contain a denial reason")
}

// TestReviewerApproval tests that comments are posted for each review, and the
// issue transitions to the approved state once it gets approved.
func (s *JiraSuiteEnterprise) TestReviewerApproval() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request
	req := s.CreateAccessRequest(ctx, integration.Requester1UserName, nil)
	pluginData := s.checkPluginData(ctx, req.GetName(), func(data jira.PluginData) bool {
		return data.IssueID != ""
	})
	issueID := pluginData.IssueID

	// Test execution: we submit a review
	err := s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	// We validate the review comment was sent
	comment, err := s.fakeJira.CheckNewIssueComment(ctx)
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "*"+integration.Reviewer1UserName+"* reviewed the request", "comment must contain a review pluginUser")

	// Test execution: we submit a second review
	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "finally okay",
	})
	require.NoError(t, err)

	// We validate the review comment was sent
	comment, err = s.fakeJira.CheckNewIssueComment(ctx)
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "*"+integration.Reviewer2UserName+"* reviewed the request", "comment must contain a review pluginUser")

	// We validate the plugin marked the request as resolved in the plugin_data.
	pluginData = s.checkPluginData(ctx, req.GetName(), func(data jira.PluginData) bool {
		return data.IssueID != "" && data.ReviewsCount == 2 && data.Resolution.Tag != jira.Unresolved
	})
	assert.Equal(t, issueID, pluginData.IssueID)
	assert.Equal(t, jira.Resolution{Tag: jira.ResolvedApproved, Reason: "finally okay"}, pluginData.Resolution)

	// We validate the issue transitioned to an approved state.
	issue, err := s.fakeJira.CheckIssueTransition(ctx)
	require.NoError(t, err, "no issue transition detected")
	assert.Equal(t, issueID, issue.ID)
	assert.Equal(t, "Approved", issue.Fields.Status.Name)

	// We validate that a final comment was sent, describing the request resolution.
	comment, err = s.fakeJira.CheckNewIssueComment(ctx)
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "Access request has been approved")
	assert.Contains(t, comment.Body, "Reason: finally okay")
}

// TestReviewerDenial tests that comments are posted for each review, and the
// issue transitions to the denied state once it gets denied.
func (s *JiraSuiteEnterprise) TestReviewerDenial() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request
	req := s.CreateAccessRequest(ctx, integration.Requester1UserName, nil)
	pluginData := s.checkPluginData(ctx, req.GetName(), func(data jira.PluginData) bool {
		return data.IssueID != ""
	})
	issueID := pluginData.IssueID

	// Test execution: we submit a review
	err := s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	// We validate the review comment was sent
	comment, err := s.fakeJira.CheckNewIssueComment(ctx)
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "*"+integration.Reviewer1UserName+"* reviewed the request", "comment must contain a review pluginUser")

	// Test execution: we submit a second review
	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "finally not okay",
	})
	require.NoError(t, err)

	// We validate the review comment was sent
	comment, err = s.fakeJira.CheckNewIssueComment(ctx)
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "*"+integration.Reviewer2UserName+"* reviewed the request", "comment must contain a review pluginUser")

	// We validate the plugin marked the request as resolved in the plugin_data.
	pluginData = s.checkPluginData(ctx, req.GetName(), func(data jira.PluginData) bool {
		return data.IssueID != "" && data.ReviewsCount == 2 && data.Resolution.Tag != jira.Unresolved
	})
	assert.Equal(t, issueID, pluginData.IssueID)
	assert.Equal(t, jira.Resolution{Tag: jira.ResolvedDenied, Reason: "finally not okay"}, pluginData.Resolution)

	// We validate the issue transitioned to a denied state.
	issue, err := s.fakeJira.CheckIssueTransition(ctx)
	require.NoError(t, err, "no issue transition detected")
	assert.Equal(t, issueID, issue.ID)
	assert.Equal(t, "Denied", issue.Fields.Status.Name)

	// We validate that a final comment was sent, describing the request resolution.
	comment, err = s.fakeJira.CheckNewIssueComment(ctx)
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "Access request has been denied")
	assert.Contains(t, comment.Body, "Reason: finally not okay")
}

// TestWebhookApproval tests that the access request plugin can approve
// requests based on Jira webhooks.
func (s *JiraSuiteOSS) TestWebhookApproval() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request
	request := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)
	pluginData := s.checkPluginData(ctx, request.GetName(), func(data jira.PluginData) bool {
		return data.IssueID != ""
	}) // when issue id is written, we are sure that request is completely served.
	issueID := pluginData.IssueID

	issue, err := s.fakeJira.CheckNewIssue(ctx)
	require.NoError(t, err, "no new issue stored")
	assert.Equal(t, issueID, issue.ID)

	// Test execution: we simulate a user approving the request in Jira
	// The issue transitions to the approved state
	s.fakeJira.TransitionIssue(issue, "Approved", s.jiraReviewerUser)
	// Jira notifies the plugin via a webhook
	s.postWebhookAndCheck(ctx, s.webhookURL.String(), issue.ID, "Approved")

	// We validate that the plugin approved the request
	request, err = s.Ruler().GetAccessRequest(ctx, request.GetName())
	require.NoError(t, err)
	assert.Equal(t, types.RequestState_APPROVED, request.GetState())

	// We validate that the approval event contains information about which
	// Jira user approved the request.
	events, err := s.Ruler().SearchAccessRequestEvents(ctx, request.GetName())
	require.NoError(t, err)
	if assert.Len(t, events, 1) {
		assert.Equal(t, "APPROVED", events[0].RequestState)
		assert.Equal(t, "jira:"+s.jiraReviewerUser.EmailAddress, events[0].Delegator)
	}

	// We validate that the plugin commented in the jira issue to confirm the
	// request was approved.
	comment, err := s.fakeJira.CheckNewIssueComment(ctx)
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "Access request has been approved")
}

// TestWebhookDenial tests that the access request plugin can deny requests
// based on Jira webhooks.
func (s *JiraSuiteOSS) TestWebhookDenial() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request
	request := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)
	pluginData := s.checkPluginData(ctx, request.GetName(), func(data jira.PluginData) bool {
		return data.IssueID != ""
	}) // when issue id is written, we are sure that request is completely served.
	issueID := pluginData.IssueID

	issue, err := s.fakeJira.CheckNewIssue(ctx)
	require.NoError(t, err, "no new issue stored")
	assert.Equal(t, issueID, issue.ID)

	// Test execution: we simulate a user denying the request in Jira
	// The issue transitions to the denied state
	s.fakeJira.TransitionIssue(issue, "Denied", s.jiraReviewerUser)
	// Jira notifies the plugin via a webhook
	s.postWebhookAndCheck(ctx, s.webhookURL.String(), issue.ID, "Denied")

	// We validate that the plugin denied the request
	request, err = s.Ruler().GetAccessRequest(ctx, request.GetName())
	require.NoError(t, err)
	assert.Equal(t, types.RequestState_DENIED, request.GetState())

	// We validate that the denial event contains information about which
	// Jira user denied the request.
	events, err := s.Ruler().SearchAccessRequestEvents(ctx, request.GetName())
	require.NoError(t, err)
	if assert.Len(t, events, 1) {
		assert.Equal(t, "DENIED", events[0].RequestState)
		assert.Equal(t, "jira:"+s.jiraReviewerUser.EmailAddress, events[0].Delegator)
	}

	// We validate that the plugin commented in the jira issue to confirm the
	// request was denied.
	comment, err := s.fakeJira.CheckNewIssueComment(ctx)
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "Access request has been denied")
}

// TestWebhookApprovalWithReason tests that the access request plugin can approve
// requests and specify the approval reason based on Jira webhooks and comments.
func (s *JiraSuiteOSS) TestWebhookApprovalWithReason() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request
	request := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)
	pluginData := s.checkPluginData(ctx, request.GetName(), func(data jira.PluginData) bool {
		return data.IssueID != ""
	})
	issueID := pluginData.IssueID

	issue, err := s.fakeJira.CheckNewIssue(ctx)
	require.NoError(t, err, "no new issue stored")
	assert.Equal(t, issueID, issue.ID)

	// Test execution: we simulate a user approving the request in Jira
	// and leaving a comment explaining why.
	issue = s.fakeJira.StoreIssueComment(issue, jira.Comment{
		Author: s.jiraReviewerUser,
		Body:   "hi! i'm going to approve this request.\nReason:\n\nfoo\nbar\nbaz",
	})
	s.fakeJira.TransitionIssue(issue, "Approved", s.jiraReviewerUser)
	s.postWebhookAndCheck(ctx, s.webhookURL.String(), issue.ID, "Approved")

	// We validate the request got approved with the reason specified in the
	// comment.
	request, err = s.Ruler().GetAccessRequest(ctx, request.GetName())
	require.NoError(t, err)
	assert.Equal(t, types.RequestState_APPROVED, request.GetState())
	assert.Equal(t, "foo\nbar\nbaz", request.GetResolveReason())

	// We validate that the approval event contains information about which
	// Jira user approved the request.
	events, err := s.Ruler().SearchAccessRequestEvents(ctx, request.GetName())
	require.NoError(t, err)
	if assert.Len(t, events, 1) {
		assert.Equal(t, "APPROVED", events[0].RequestState)
		assert.Equal(t, "jira:"+s.jiraReviewerUser.EmailAddress, events[0].Delegator)
	}

	// We validate that the plugin commented in the jira issue to confirm the
	// request was approved.
	comment, err := s.fakeJira.CheckNewIssueComment(ctx)
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "Access request has been approved")
	assert.Contains(t, comment.Body, "Reason: foo\nbar\nbaz")
}

// TestWebhookDenialWithReason tests that the access request plugin can deny
// requests and specify the denial reason based on Jira webhooks and comments.
// This test has extra cases to validate that the reason is picked from the
// correct Jira comment.
func (s *JiraSuiteOSS) TestWebhookDenialWithReason() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request
	request := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)
	pluginData := s.checkPluginData(ctx, request.GetName(), func(data jira.PluginData) bool {
		return data.IssueID != ""
	})
	issueID := pluginData.IssueID

	issue, err := s.fakeJira.CheckNewIssue(ctx)
	require.NoError(t, err, "no new issue stored")
	assert.Equal(t, issueID, issue.ID)

	// Test execution: we simulate a user deny the request in Jira
	// and leaving a comment explaining why. We simulate a discussion in the
	// comments, this will allow us to validate that the plugin picks the right
	// comment.

	// Comment with the reason that will be superseded
	issue = s.fakeJira.StoreIssueComment(issue, jira.Comment{
		Author: s.jiraReviewerUser,
		Body:   "hi! i'm rejecting the request.\nreason: bar baz", // reason is "bar baz" but the next comment will override it.
	})
	// Newer comment with the reason, this one should be picked
	issue = s.fakeJira.StoreIssueComment(issue, jira.Comment{
		Author: s.jiraReviewerUser,
		Body:   "hi! i'm rejecting the request.\nreason: foo bar baz", // reason is "foo bar baz".
	})
	// Comment without the reason pattern, it should be ignored
	issue = s.fakeJira.StoreIssueComment(issue, jira.Comment{
		Author: s.jiraReviewerUser,
		Body:   "comment 1", // just ignored.
	})
	// Comment with a reason, but from another user that the one who did the approval.
	// Should be ignored
	issue = s.fakeJira.StoreIssueComment(issue, jira.Comment{
		Author: s.jiraOtherUser,
		Body:   "reason: test",
	})
	s.fakeJira.TransitionIssue(issue, "Denied", s.jiraReviewerUser)
	s.postWebhookAndCheck(ctx, s.webhookURL.String(), issue.ID, "Denied")

	// We validate the request got denied with the reason specified in the comment.
	request, err = s.Ruler().GetAccessRequest(ctx, request.GetName())
	require.NoError(t, err)
	assert.Equal(t, types.RequestState_DENIED, request.GetState())
	assert.Equal(t, "foo bar baz", request.GetResolveReason())

	// We validate that the denial event contains information about which
	// Jira user denied the request.
	events, err := s.Ruler().SearchAccessRequestEvents(ctx, request.GetName())
	require.NoError(t, err)
	if assert.Len(t, events, 1) {
		assert.Equal(t, "DENIED", events[0].RequestState)
		assert.Equal(t, "jira:"+s.jiraReviewerUser.EmailAddress, events[0].Delegator)
	}

	// We validate that the plugin commented in the jira issue to confirm the
	// request was denied.
	comment, err := s.fakeJira.CheckNewIssueComment(ctx)
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "Access request has been denied")
	assert.Contains(t, comment.Body, "Reason: foo bar baz")
}

// TestExpiration tests that when a request expires, its corresponding issue
// is updated to reflect the new request state.
func (s *JiraSuiteOSS) TestExpiration() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request
	request := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)
	pluginData := s.checkPluginData(ctx, request.GetName(), func(data jira.PluginData) bool {
		return data.IssueID != ""
	})
	issueID := pluginData.IssueID

	issue, err := s.fakeJira.CheckNewIssue(ctx)
	require.NoError(t, err, "no new issue stored")
	assert.Equal(t, issueID, issue.ID)

	// Test execution: we expire the access request
	err = s.Ruler().DeleteAccessRequest(ctx, request.GetName()) // simulate expiration
	require.NoError(t, err)

	// We validate that the access request transitioned to the expired state.
	issue, err = s.fakeJira.CheckIssueTransition(ctx)
	require.NoError(t, err, "no issue transition detected")
	assert.Equal(t, issueID, issue.ID)
	assert.Equal(t, "Expired", issue.Fields.Status.Name)

	// We validate that a final comment was sent, describing the request resolution.
	comment, err := s.fakeJira.CheckNewIssueComment(ctx)
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "Access request has been expired")
}

// TestRace validates that the plugin behaves properly, sends all the comments
// and performs all the resolutions when a lot of access requests are sent and
// reviewed in a very short time frame.
func (s *JiraSuiteOSS) TestRace() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	err := logger.Setup(logger.Config{Severity: "info"}) // Turn off noisy debug logging
	require.NoError(t, err)

	s.startApp()

	var (
		raceErr     error
		raceErrOnce sync.Once
		requests    sync.Map
	)
	setRaceErr := func(err error) error {
		raceErrOnce.Do(func() {
			raceErr = err
		})
		return err
	}

	watcher, err := s.Ruler().NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{
			{
				Kind: types.KindAccessRequest,
			},
		},
	})
	require.NoError(t, err)
	defer watcher.Close()
	assert.Equal(t, types.OpInit, (<-watcher.Events()).Type)

	process := lib.NewProcess(ctx)
	for i := 0; i < s.raceNumber; i++ {
		process.SpawnCritical(func(ctx context.Context) error {
			req, err := types.NewAccessRequest(uuid.New().String(), integration.RequesterOSSUserName, "editor")
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			_, err = s.RequesterOSS().CreateAccessRequestV2(ctx, req)
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			return nil
		})
		process.SpawnCritical(func(ctx context.Context) error {
			issue, err := s.fakeJira.CheckNewIssue(ctx)
			if err := trace.Wrap(err); err != nil {
				return setRaceErr(err)
			}
			if obtained, expected := issue.Fields.Status.Name, "Pending"; obtained != expected {
				return setRaceErr(trace.Errorf("wrong issue status. expected %s, obtained %s", expected, obtained))
			}
			s.fakeJira.TransitionIssue(issue, "Approved", s.jiraReviewerUser)

			ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			var lastErr error
			for {
				logger.Get(ctx).InfoContext(ctx, "Trying to approve issue", "issue_key", issue.Key)
				resp, err := s.postWebhook(ctx, s.webhookURL.String(), issue.ID, "Approved")
				if err != nil {
					if lib.IsDeadline(err) {
						return setRaceErr(lastErr)
					}
					return setRaceErr(trace.Wrap(err))
				}
				if err := resp.Body.Close(); err != nil {
					return setRaceErr(trace.Wrap(err))
				}
				if status := resp.StatusCode; status != http.StatusOK {
					lastErr = trace.Errorf("got %v http code from webhook server", status)
				} else {
					return nil
				}
			}
		})
		process.SpawnCritical(func(ctx context.Context) error {
			issue, err := s.fakeJira.CheckIssueTransition(ctx)
			if err := trace.Wrap(err); err != nil {
				return setRaceErr(err)
			}
			if obtained, expected := issue.Fields.Status.Name, "Approved"; obtained != expected {
				return setRaceErr(trace.Errorf("wrong issue status. expected %q, obtained %q", expected, obtained))
			}
			return nil
		})
	}
	for i := 0; i < 2*s.raceNumber; i++ {
		process.SpawnCritical(func(ctx context.Context) error {
			var event types.Event
			select {
			case event = <-watcher.Events():
			case <-ctx.Done():
				return setRaceErr(trace.Wrap(ctx.Err()))
			}
			if obtained, expected := event.Type, types.OpPut; obtained != expected {
				return setRaceErr(trace.Errorf("wrong event type. expected %v, obtained %v", expected, obtained))
			}
			req := event.Resource.(types.AccessRequest)
			var newCounter int64
			val, _ := requests.LoadOrStore(req.GetName(), &newCounter)
			switch state := req.GetState(); state {
			case types.RequestState_PENDING:
				atomic.AddInt64(val.(*int64), 1)
			case types.RequestState_APPROVED:
				atomic.AddInt64(val.(*int64), -1)
			default:
				return setRaceErr(trace.Errorf("wrong request state %v", state))
			}
			return nil
		})
	}
	process.Terminate()
	<-process.Done()
	require.NoError(t, raceErr)

	var count int
	requests.Range(func(key, val interface{}) bool {
		count++
		assert.Equal(t, int64(0), *val.(*int64))
		return true
	})
	assert.Equal(t, s.raceNumber, count)
}
