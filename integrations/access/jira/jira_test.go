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

package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os/user"
	"regexp"
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
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
)

type JiraSuite struct {
	integration.Suite
	appConfig Config
	userNames struct {
		ruler     string
		requestor string
		reviewer1 string
		reviewer2 string
		plugin    string
	}
	raceNumber int
	authorUser UserDetails
	otherUser  UserDetails
	fakeJira   *FakeJira

	clients          map[string]*integration.Client
	teleportFeatures *proto.Features
	teleportConfig   lib.TeleportConfig
}

func TestJira(t *testing.T) { suite.Run(t, &JiraSuite{}) }

func (s *JiraSuite) SetupSuite() {
	var err error
	t := s.T()

	logger.Init()
	err = logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)
	s.raceNumber = runtime.GOMAXPROCS(0)
	me, err := user.Current()
	require.NoError(t, err)

	// We set such a big timeout because integration.NewFromEnv could start
	// downloading a Teleport *-bin.tar.gz file which can take a long time.
	ctx := s.SetContextTimeout(2 * time.Minute)

	teleport, err := integration.NewFromEnv(ctx)
	require.NoError(t, err)
	t.Cleanup(teleport.Close)

	auth, err := teleport.NewAuthService()
	require.NoError(t, err)
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

	conditions := types.RoleConditions{Request: &types.AccessRequestConditions{Roles: []string{"editor"}}}
	if teleportFeatures.AdvancedAccessWorkflows {
		conditions.Request.Thresholds = []types.AccessReviewThreshold{{Approve: 2, Deny: 2}}
	}
	role, err := bootstrap.AddRole("foo", types.RoleSpecV6{Allow: conditions})
	require.NoError(t, err)

	user, err := bootstrap.AddUserWithRoles(me.Username+"@example.com", role.GetName())
	require.NoError(t, err)
	s.userNames.requestor = user.GetName()

	s.authorUser = UserDetails{AccountID: "USER-1", DisplayName: me.Username, EmailAddress: s.userNames.requestor}
	s.otherUser = UserDetails{AccountID: "USER-2", DisplayName: me.Username + " evil twin", EmailAddress: me.Username + "-evil@example.com"}

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
	}

	// Set up plugin user.

	role, err = bootstrap.AddRole("access-jira", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule("access_request", []string{"list", "read", "update"}),
			},
		},
	})
	require.NoError(t, err)

	user, err = bootstrap.AddUserWithRoles("access-jira", role.GetName())
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
		client, err = teleport.NewClient(ctx, auth, s.userNames.reviewer1)
		require.NoError(t, err)
		s.clients[s.userNames.reviewer1] = client

		client, err = teleport.NewClient(ctx, auth, s.userNames.reviewer2)
		require.NoError(t, err)
		s.clients[s.userNames.reviewer2] = client
	}

	identityPath, err := teleport.Sign(ctx, auth, s.userNames.plugin)
	require.NoError(t, err)

	s.teleportConfig.Addr = auth.AuthAddr().String()
	s.teleportConfig.Identity = identityPath
	s.teleportFeatures = teleportFeatures
}

func (s *JiraSuite) SetupTest() {
	t := s.T()

	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)

	s.fakeJira = NewFakeJira(s.authorUser, s.raceNumber)
	t.Cleanup(s.fakeJira.Close)

	var conf Config
	conf.Teleport = s.teleportConfig
	conf.Jira.URL = s.fakeJira.URL()
	conf.Jira.Username = "jira-bot@example.com"
	conf.Jira.APIToken = "xyz"
	conf.Jira.Project = "PROJ"
	conf.HTTP.ListenAddr = ":0"
	conf.HTTP.Insecure = true

	s.appConfig = conf
	s.SetContextTimeout(5 * time.Second)
}

func (s *JiraSuite) startApp() *App {
	t := s.T()
	t.Helper()

	app, err := NewApp(s.appConfig)
	require.NoError(t, err)

	s.StartApp(app)

	return app
}

func (s *JiraSuite) ruler() *integration.Client {
	return s.clients[s.userNames.ruler]
}

func (s *JiraSuite) requestor() *integration.Client {
	return s.clients[s.userNames.requestor]
}

func (s *JiraSuite) reviewer1() *integration.Client {
	return s.clients[s.userNames.reviewer1]
}

func (s *JiraSuite) reviewer2() *integration.Client {
	return s.clients[s.userNames.reviewer2]
}

func (s *JiraSuite) newAccessRequest() types.AccessRequest {
	t := s.T()
	t.Helper()

	req, err := types.NewAccessRequest(uuid.New().String(), s.userNames.requestor, "editor")
	require.NoError(t, err)
	return req
}

func (s *JiraSuite) createAccessRequest() types.AccessRequest {
	t := s.T()
	t.Helper()

	req := s.newAccessRequest()
	err := s.requestor().CreateAccessRequest(s.Context(), req)
	require.NoError(t, err)
	return req
}

func (s *JiraSuite) checkPluginData(reqID string, cond func(PluginData) bool) PluginData {
	t := s.T()
	t.Helper()

	for {
		rawData, err := s.ruler().PollAccessRequestPluginData(s.Context(), "jira", reqID)
		require.NoError(t, err)
		if data := DecodePluginData(rawData); cond(data) {
			return data
		}
	}
}

func (s *JiraSuite) postWebhook(ctx context.Context, url, issueID string) (*http.Response, error) {
	var buf bytes.Buffer
	wh := Webhook{
		WebhookEvent:       "jira:issue_updated",
		IssueEventTypeName: "issue_generic",
		Issue:              &WebhookIssue{ID: issueID},
	}
	err := json.NewEncoder(&buf).Encode(&wh)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	request.Header.Add("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	return response, trace.Wrap(err)
}

func (s *JiraSuite) postWebhookAndCheck(url, issueID string) {
	t := s.T()
	t.Helper()

	resp, err := s.postWebhook(s.Context(), url, issueID)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func (s *JiraSuite) TestIssueCreation() {
	t := s.T()

	s.startApp()
	request := s.createAccessRequest()

	pluginData := s.checkPluginData(request.GetName(), func(data PluginData) bool {
		return data.IssueID != ""
	}) // when issue id is written, we are sure that request is completely served.

	issue, err := s.fakeJira.CheckNewIssue(s.Context())
	require.NoError(t, err, "no new issue stored")
	assert.Equal(t, "PROJ", issue.Fields.Project.Key)
	assert.Equal(t, request.GetName(), issue.Properties[RequestIDPropertyKey])
	assert.Equal(t, pluginData.IssueID, issue.ID)
}

func (s *JiraSuite) TestIssueCreationWithRequestReason() {
	t := s.T()

	s.startApp()

	req := s.newAccessRequest()
	req.SetRequestReason("because of")
	err := s.requestor().CreateAccessRequest(s.Context(), req)
	require.NoError(t, err)
	s.checkPluginData(req.GetName(), func(data PluginData) bool {
		return data.IssueID != ""
	}) // when issue id is written, we are sure that request is completely served.

	issue, err := s.fakeJira.CheckNewIssue(s.Context())
	require.NoError(t, err)

	if !strings.Contains(issue.Fields.Description, `Reason: *because of*`) {
		t.Error("Issue description should contain request reason")
	}
}

func (s *JiraSuite) TestIssueCreationWithLargeRequestReason() {
	t := s.T()

	s.startApp()

	req := s.newAccessRequest()
	req.SetRequestReason(strings.Repeat("a", jiraReasonLimit+10))
	err := s.requestor().CreateAccessRequest(s.Context(), req)
	require.NoError(t, err)
	s.checkPluginData(req.GetName(), func(data PluginData) bool {
		return data.IssueID != ""
	}) // when issue id is written, we are sure that request is completely served.

	issue, err := s.fakeJira.CheckNewIssue(s.Context())
	require.NoError(t, err)
	re := regexp.MustCompile("(?:Reason...)(a+)")
	match := re.FindStringSubmatch(issue.Fields.Description)
	if len(match) != 2 {
		t.Error("reason not found in issue description")
		return
	}
	require.Equal(t, jiraReasonLimit, len(match[1]))
}

func (s *JiraSuite) TestReviewComments() {
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
		return data.IssueID != "" && data.ReviewsCount == 2
	})

	comment, err := s.fakeJira.CheckNewIssueComment(s.Context())
	require.NoError(t, err)
	assert.Equal(t, pluginData.IssueID, comment.IssueID)
	assert.Contains(t, comment.Body, "*"+s.userNames.reviewer1+"* reviewed the request", "comment must contain a review author")
	assert.Contains(t, comment.Body, "Resolution: *APPROVED*", "comment must contain an approval resolution")
	assert.Contains(t, comment.Body, "Reason: okay", "comment must contain an approval reason")

	comment, err = s.fakeJira.CheckNewIssueComment(s.Context())
	require.NoError(t, err)
	assert.Equal(t, pluginData.IssueID, comment.IssueID)
	assert.Contains(t, comment.Body, "*"+s.userNames.reviewer2+"* reviewed the request", "comment must contain a review author")
	assert.Contains(t, comment.Body, "Resolution: *DENIED*", "comment must contain a denial resolution")
	assert.Contains(t, comment.Body, "Reason: not okay", "comment must contain a denial reason")
}

func (s *JiraSuite) TestReviewerApproval() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	s.startApp()
	req := s.createAccessRequest()

	pluginData := s.checkPluginData(req.GetName(), func(data PluginData) bool {
		return data.IssueID != ""
	})
	issueID := pluginData.IssueID

	err := s.reviewer1().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer1,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	comment, err := s.fakeJira.CheckNewIssueComment(s.Context())
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "*"+s.userNames.reviewer1+"* reviewed the request", "comment must contain a review author")

	err = s.reviewer2().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer2,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "finally okay",
	})
	require.NoError(t, err)

	comment, err = s.fakeJira.CheckNewIssueComment(s.Context())
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "*"+s.userNames.reviewer2+"* reviewed the request", "comment must contain a review author")

	pluginData = s.checkPluginData(req.GetName(), func(data PluginData) bool {
		return data.IssueID != "" && data.ReviewsCount == 2 && data.Resolution.Tag != Unresolved
	})
	assert.Equal(t, issueID, pluginData.IssueID)
	assert.Equal(t, Resolution{Tag: ResolvedApproved, Reason: "finally okay"}, pluginData.Resolution)

	issue, err := s.fakeJira.CheckIssueTransition(s.Context())
	require.NoError(t, err, "no issue transition detected")
	assert.Equal(t, issueID, issue.ID)
	assert.Equal(t, "Approved", issue.Fields.Status.Name)

	comment, err = s.fakeJira.CheckNewIssueComment(s.Context())
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "Access request has been approved")
	assert.Contains(t, comment.Body, "Reason: finally okay")
}

func (s *JiraSuite) TestReviewerDenial() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	s.startApp()
	req := s.createAccessRequest()

	pluginData := s.checkPluginData(req.GetName(), func(data PluginData) bool {
		return data.IssueID != ""
	})
	issueID := pluginData.IssueID

	err := s.reviewer1().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer1,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	comment, err := s.fakeJira.CheckNewIssueComment(s.Context())
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "*"+s.userNames.reviewer1+"* reviewed the request", "comment must contain a review author")

	err = s.reviewer2().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer2,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "finally not okay",
	})
	require.NoError(t, err)

	comment, err = s.fakeJira.CheckNewIssueComment(s.Context())
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "*"+s.userNames.reviewer2+"* reviewed the request", "comment must contain a review author")

	pluginData = s.checkPluginData(req.GetName(), func(data PluginData) bool {
		return data.IssueID != "" && data.ReviewsCount == 2 && data.Resolution.Tag != Unresolved
	})
	assert.Equal(t, issueID, pluginData.IssueID)
	assert.Equal(t, Resolution{Tag: ResolvedDenied, Reason: "finally not okay"}, pluginData.Resolution)

	issue, err := s.fakeJira.CheckIssueTransition(s.Context())
	require.NoError(t, err, "no issue transition detected")
	assert.Equal(t, issueID, issue.ID)
	assert.Equal(t, "Denied", issue.Fields.Status.Name)

	comment, err = s.fakeJira.CheckNewIssueComment(s.Context())
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "Access request has been denied")
	assert.Contains(t, comment.Body, "Reason: finally not okay")
}

func (s *JiraSuite) TestWebhookApproval() {
	t := s.T()

	app := s.startApp()

	request := s.createAccessRequest()
	pluginData := s.checkPluginData(request.GetName(), func(data PluginData) bool {
		return data.IssueID != ""
	}) // when issue id is written, we are sure that request is completely served.
	issueID := pluginData.IssueID

	issue, err := s.fakeJira.CheckNewIssue(s.Context())
	require.NoError(t, err, "no new issue stored")
	assert.Equal(t, issueID, issue.ID)

	s.fakeJira.TransitionIssue(issue, "Approved")
	s.postWebhookAndCheck(app.PublicURL().String(), issue.ID)

	request, err = s.ruler().GetAccessRequest(s.Context(), request.GetName())
	require.NoError(t, err)
	assert.Equal(t, types.RequestState_APPROVED, request.GetState())

	events, err := s.ruler().SearchAccessRequestEvents(s.Context(), request.GetName())
	require.NoError(t, err)
	if assert.Len(t, events, 1) {
		assert.Equal(t, "APPROVED", events[0].RequestState)
		assert.Equal(t, "jira:"+s.authorUser.EmailAddress, events[0].Delegator)
	}

	comment, err := s.fakeJira.CheckNewIssueComment(s.Context())
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "Access request has been approved")
}

func (s *JiraSuite) TestWebhookDenial() {
	t := s.T()

	app := s.startApp()

	request := s.createAccessRequest()
	pluginData := s.checkPluginData(request.GetName(), func(data PluginData) bool {
		return data.IssueID != ""
	}) // when issue id is written, we are sure that request is completely served.
	issueID := pluginData.IssueID

	issue, err := s.fakeJira.CheckNewIssue(s.Context())
	require.NoError(t, err, "no new issue stored")
	assert.Equal(t, issueID, issue.ID)

	s.fakeJira.TransitionIssue(issue, "Denied")
	s.postWebhookAndCheck(app.PublicURL().String(), issue.ID)

	request, err = s.ruler().GetAccessRequest(s.Context(), request.GetName())
	require.NoError(t, err)
	assert.Equal(t, types.RequestState_DENIED, request.GetState())

	events, err := s.ruler().SearchAccessRequestEvents(s.Context(), request.GetName())
	require.NoError(t, err)
	if assert.Len(t, events, 1) {
		assert.Equal(t, "DENIED", events[0].RequestState)
		assert.Equal(t, "jira:"+s.authorUser.EmailAddress, events[0].Delegator)
	}

	comment, err := s.fakeJira.CheckNewIssueComment(s.Context())
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "Access request has been denied")
}

func (s *JiraSuite) TestWebhookApprovalWithReason() {
	t := s.T()

	app := s.startApp()

	request := s.createAccessRequest()
	pluginData := s.checkPluginData(request.GetName(), func(data PluginData) bool {
		return data.IssueID != ""
	})
	issueID := pluginData.IssueID

	issue, err := s.fakeJira.CheckNewIssue(s.Context())
	require.NoError(t, err, "no new issue stored")
	assert.Equal(t, issueID, issue.ID)

	issue = s.fakeJira.StoreIssueComment(issue, Comment{
		Author: s.authorUser,
		Body:   "hi! i'm going to approve this request.\nReason:\n\nfoo\nbar\nbaz",
	})

	s.fakeJira.TransitionIssue(issue, "Approved")
	s.postWebhookAndCheck(app.PublicURL().String(), issue.ID)

	request, err = s.ruler().GetAccessRequest(s.Context(), request.GetName())
	require.NoError(t, err)
	assert.Equal(t, types.RequestState_APPROVED, request.GetState())
	assert.Equal(t, "foo\nbar\nbaz", request.GetResolveReason())

	events, err := s.ruler().SearchAccessRequestEvents(s.Context(), request.GetName())
	require.NoError(t, err)
	if assert.Len(t, events, 1) {
		assert.Equal(t, "APPROVED", events[0].RequestState)
		assert.Equal(t, "jira:"+s.authorUser.EmailAddress, events[0].Delegator)
	}

	comment, err := s.fakeJira.CheckNewIssueComment(s.Context())
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "Access request has been approved")
	assert.Contains(t, comment.Body, "Reason: foo\nbar\nbaz")
}

func (s *JiraSuite) TestWebhookDenialWithReason() {
	t := s.T()

	app := s.startApp()

	request := s.createAccessRequest()
	pluginData := s.checkPluginData(request.GetName(), func(data PluginData) bool {
		return data.IssueID != ""
	})
	issueID := pluginData.IssueID

	issue, err := s.fakeJira.CheckNewIssue(s.Context())
	require.NoError(t, err, "no new issue stored")
	assert.Equal(t, issueID, issue.ID)

	issue = s.fakeJira.StoreIssueComment(issue, Comment{
		Author: s.otherUser,
		Body:   "comment 1", // just ignored.
	})
	issue = s.fakeJira.StoreIssueComment(issue, Comment{
		Author: s.authorUser,
		Body:   "hi! i'm rejecting the request.\nreason: bar baz", // reason is "bar baz" but the next comment will override it.
	})
	issue = s.fakeJira.StoreIssueComment(issue, Comment{
		Author: s.authorUser,
		Body:   "hi! i'm rejecting the request.\nreason: foo bar baz", // reason is "foo bar baz".
	})
	issue = s.fakeJira.StoreIssueComment(issue, Comment{
		Author: s.otherUser,
		Body:   "reason: test", // has reason too but ignored because it's not the same user that did transition.
	})

	s.fakeJira.TransitionIssue(issue, "Denied")
	s.postWebhookAndCheck(app.PublicURL().String(), issue.ID)

	request, err = s.ruler().GetAccessRequest(s.Context(), request.GetName())
	require.NoError(t, err)
	assert.Equal(t, types.RequestState_DENIED, request.GetState())
	assert.Equal(t, "foo bar baz", request.GetResolveReason())

	events, err := s.ruler().SearchAccessRequestEvents(s.Context(), request.GetName())
	require.NoError(t, err)
	if assert.Len(t, events, 1) {
		assert.Equal(t, "DENIED", events[0].RequestState)
		assert.Equal(t, "jira:"+s.authorUser.EmailAddress, events[0].Delegator)
	}

	comment, err := s.fakeJira.CheckNewIssueComment(s.Context())
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "Access request has been denied")
	assert.Contains(t, comment.Body, "Reason: foo bar baz")
}

func (s *JiraSuite) TestExpiration() {
	t := s.T()

	s.startApp()

	request := s.createAccessRequest()
	pluginData := s.checkPluginData(request.GetName(), func(data PluginData) bool {
		return data.IssueID != ""
	})
	issueID := pluginData.IssueID

	issue, err := s.fakeJira.CheckNewIssue(s.Context())
	require.NoError(t, err, "no new issue stored")
	assert.Equal(t, issueID, issue.ID)

	err = s.ruler().DeleteAccessRequest(s.Context(), request.GetName()) // simulate expiration
	require.NoError(t, err)

	issue, err = s.fakeJira.CheckIssueTransition(s.Context())
	require.NoError(t, err, "no issue transition detected")
	assert.Equal(t, issueID, issue.ID)
	assert.Equal(t, "Expired", issue.Fields.Status.Name)

	comment, err := s.fakeJira.CheckNewIssueComment(s.Context())
	require.NoError(t, err)
	assert.Equal(t, issueID, comment.IssueID)
	assert.Contains(t, comment.Body, "Access request has been expired")
}

func (s *JiraSuite) TestRace() {
	t := s.T()

	err := logger.Setup(logger.Config{Severity: "info"}) // Turn off noisy debug logging
	require.NoError(t, err)

	s.SetContextTimeout(20 * time.Second)
	app := s.startApp()

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

	watcher, err := s.ruler().NewWatcher(s.Context(), types.Watch{
		Kinds: []types.WatchKind{
			{
				Kind: types.KindAccessRequest,
			},
		},
	})
	require.NoError(t, err)
	defer watcher.Close()
	assert.Equal(t, types.OpInit, (<-watcher.Events()).Type)

	process := lib.NewProcess(s.Context())
	for i := 0; i < s.raceNumber; i++ {
		process.SpawnCritical(func(ctx context.Context) error {
			req, err := types.NewAccessRequest(uuid.New().String(), s.userNames.requestor, "editor")
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			if err = s.requestor().CreateAccessRequest(s.Context(), req); err != nil {
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
			s.fakeJira.TransitionIssue(issue, "Approved")

			ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			var lastErr error
			for {
				logger.Get(ctx).Infof("Trying to approve issue %q", issue.Key)
				resp, err := s.postWebhook(ctx, app.PublicURL().String(), issue.ID)
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
