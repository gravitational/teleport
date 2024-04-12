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
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/accessrequest"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/mattermost"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
)

var msgFieldRegexp = regexp.MustCompile(`(?im)^\*\*([a-zA-Z ]+)\*\*:\ +(.+)$`)
var requestReasonRegexp = regexp.MustCompile("(?im)^\\*\\*Reason\\*\\*:\\ ```\\n(.*?)```(.*?)$")
var resolutionReasonRegexp = regexp.MustCompile("(?im)^\\*\\*Resolution reason\\*\\*:\\ ```\\n(.*?)```(.*?)$")

// MattermostBaseSuite is the Mattermost access plugin test suite.
// It implements the testify.TestingSuite interface.
type MattermostBaseSuite struct {
	*integration.AccessRequestSuite
	appConfig      *mattermost.Config
	raceNumber     int
	fakeMattermost *FakeMattermost
	fakeStatusSink *integration.FakeStatusSink

	requesterMattermostUser mattermost.User
	reviewer1MattermostUser mattermost.User
	reviewer2MattermostUser mattermost.User
}

// SetupTest starts a fake Mattermost and generates the plugin configuration.
// It is run for each test.
func (s *MattermostBaseSuite) SetupTest() {
	t := s.T()

	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)
	s.raceNumber = runtime.GOMAXPROCS(0)

	s.fakeMattermost = NewFakeMattermost(mattermost.User{Username: "bot", Email: "bot@example.com"}, s.raceNumber)
	t.Cleanup(s.fakeMattermost.Close)

	// load fixtures in the fake Mattermost
	s.requesterMattermostUser = s.fakeMattermost.StoreUser(mattermost.User{Email: integration.Requester1UserName})
	s.reviewer1MattermostUser = s.fakeMattermost.StoreUser(mattermost.User{Email: integration.Reviewer1UserName})
	s.reviewer2MattermostUser = s.fakeMattermost.StoreUser(mattermost.User{Email: integration.Reviewer2UserName})

	s.fakeStatusSink = &integration.FakeStatusSink{}

	var conf mattermost.Config
	conf.Teleport = s.TeleportConfig()
	conf.Mattermost.Token = "000000"
	conf.Mattermost.URL = s.fakeMattermost.URL()
	conf.StatusSink = s.fakeStatusSink

	s.appConfig = &conf
}

// startApp starts the Mattermost plugin, waits for it to become ready and returns,
func (s *MattermostBaseSuite) startApp() {
	t := s.T()
	t.Helper()

	app := mattermost.NewMattermostApp(s.appConfig)
	integration.RunAndWaitReady(t, app)
}

// MattermostSuiteOSS contains all tests that support running against a Teleport
// OSS Server.
type MattermostSuiteOSS struct {
	MattermostBaseSuite
}

// MattermostSuiteEnterprise contains all tests that require a Teleport Enterprise
// to run.
type MattermostSuiteEnterprise struct {
	MattermostBaseSuite
}

// SetupTest overrides MattermostBaseSuite.SetupTest to check the Teleport features
// before each test.
func (s *MattermostSuiteEnterprise) SetupTest() {
	t := s.T()
	s.RequireAdvancedWorkflow(t)
	s.MattermostBaseSuite.SetupTest()
}

// TestMattermostMessagePosting validates that a message is sent to each recipient
// specified in the plugin's configuration and optional reviewers.
// It also checks that the message content is correct, even when the reason
// is too large.
func (s *MattermostSuiteOSS) TestMattermostMessagePosting() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// max size of request was decreased here: https://github.com/gravitational/teleport/pull/13298
	s.SetReasonPadding(4000)

	// Reviewer 1 is a recipient because they're in the config
	s.appConfig.Recipients = common.RawRecipientsMap{
		integration.RequestedRoleName: []string{
			s.reviewer1MattermostUser.Email,
		},
		"*": []string{"fallback"},
	}

	directChannel1 := s.fakeMattermost.GetDirectChannelFor(s.fakeMattermost.GetBotUser(), s.reviewer1MattermostUser)
	directChannel2 := s.fakeMattermost.GetDirectChannelFor(s.fakeMattermost.GetBotUser(), s.reviewer2MattermostUser)

	s.startApp()

	// Test execution: we send an access request
	userName := integration.RequesterOSSUserName
	// Reviewer 2 is a recipient because they're in the suggested reviewers
	req := s.CreateAccessRequest(ctx, userName, []string{s.reviewer2MattermostUser.Email})

	// We expect 2 messages to be sent by the plugin: one for each recipient.
	// We check if the stored plugin data makes sense.
	pluginData := s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})
	assert.Len(t, pluginData.SentMessages, 2)

	// Then we check that our fake Mattermost has received the messages.
	var posts []mattermost.Post
	postSet := make(MattermostDataPostSet)
	for i := 0; i < 2; i++ {
		post, err := s.fakeMattermost.CheckNewPost(ctx)
		require.NoError(t, err, "no new messages posted")
		postSet.Add(accessrequest.MessageData{ChannelID: post.ChannelID, MessageID: post.ID})
		posts = append(posts, post)
	}

	assert.Len(t, postSet, 2)
	assert.Contains(t, postSet, pluginData.SentMessages[0])
	assert.Contains(t, postSet, pluginData.SentMessages[1])

	// Finally, we validate the messages content
	sort.Sort(MattermostPostSlice(posts))
	assert.Equal(t, directChannel1.ID, posts[0].ChannelID)
	assert.Equal(t, directChannel2.ID, posts[1].ChannelID)

	post := posts[0]
	reqID, err := parsePostField(post, "Request ID")
	require.NoError(t, err)
	assert.Equal(t, req.GetName(), reqID)

	username, err := parsePostField(post, "User")
	require.NoError(t, err)
	assert.Equal(t, integration.RequesterOSSUserName, username)

	matches := requestReasonRegexp.FindAllStringSubmatch(post.Message, -1)
	require.Len(t, matches, 1)
	require.Len(t, matches[0], 3)
	assert.Equal(t, "because of "+strings.Repeat("A", 489), matches[0][1])
	assert.Equal(t, " (truncated)", matches[0][2])

	statusLine, err := parsePostField(post, "Status")
	require.NoError(t, err)
	assert.Equal(t, "⏳ PENDING", statusLine)
	assert.Equal(t, types.PluginStatusCode_RUNNING, s.fakeStatusSink.Get().GetCode())
}

// TestApproval tests that when a request is approved, its corresponding message
// is updated to reflect the new request state.
func (s *MattermostSuiteOSS) TestApproval() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its message
	userName := integration.RequesterOSSUserName
	req := s.CreateAccessRequest(ctx, userName, []string{s.reviewer1MattermostUser.Email})

	post, err := s.fakeMattermost.CheckNewPost(ctx)
	require.NoError(t, err, "no new messages posted")
	directChannelID := s.fakeMattermost.GetDirectChannelFor(s.fakeMattermost.GetBotUser(), s.reviewer1MattermostUser).ID
	assert.Equal(t, directChannelID, post.ChannelID)

	// Test execution: we approve the request
	err = s.Ruler().ApproveAccessRequest(ctx, req.GetName(), "okay")
	require.NoError(t, err)

	// Validating the plugin updated the message to reflect that it got approved
	postUpdate, err := s.fakeMattermost.CheckPostUpdate(ctx)
	require.NoError(t, err, "no messages updated")
	assert.Equal(t, post.ID, postUpdate.ID)
	assert.Equal(t, post.ChannelID, postUpdate.ChannelID)

	statusLine, err := parsePostField(postUpdate, "Status")
	require.NoError(t, err)
	assert.Equal(t, "✅ APPROVED", statusLine)

	matches := resolutionReasonRegexp.FindAllStringSubmatch(postUpdate.Message, -1)
	require.Len(t, matches, 1)
	require.Len(t, matches[0], 3)
	assert.Equal(t, "okay", matches[0][1])
	assert.Equal(t, "", matches[0][2])
}

// TestDenial tests that when a request is denied, its corresponding message
// is updated to reflect the new request state.
func (s *MattermostSuiteOSS) TestDenial() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its message
	userName := integration.RequesterOSSUserName
	req := s.CreateAccessRequest(ctx, userName, []string{s.reviewer1MattermostUser.Email})

	post, err := s.fakeMattermost.CheckNewPost(ctx)
	require.NoError(t, err, "no new messages posted")
	directChannelID := s.fakeMattermost.GetDirectChannelFor(s.fakeMattermost.GetBotUser(), s.reviewer1MattermostUser).ID
	assert.Equal(t, directChannelID, post.ChannelID)

	// Test execution: we approve the request
	err = s.Ruler().DenyAccessRequest(ctx, req.GetName(), "not okay "+strings.Repeat("A", 4000))
	require.NoError(t, err)

	// Validating the plugin updated the message to reflect that it got denied
	postUpdate, err := s.fakeMattermost.CheckPostUpdate(ctx)
	require.NoError(t, err, "no messages updated")
	assert.Equal(t, post.ID, postUpdate.ID)
	assert.Equal(t, post.ChannelID, postUpdate.ChannelID)

	statusLine, err := parsePostField(postUpdate, "Status")
	require.NoError(t, err)
	assert.Equal(t, "❌ DENIED", statusLine)

	matches := resolutionReasonRegexp.FindAllStringSubmatch(postUpdate.Message, -1)
	require.Len(t, matches, 1)
	require.Len(t, matches[0], 3)
	assert.Equal(t, "not okay "+strings.Repeat("A", 491), matches[0][1])
	assert.Equal(t, " (truncated)", matches[0][2])
}

// TestReviewComments tests that that update messages are sent after the access
// request is reviewed. Each review should trigger a new message.
func (s *MattermostSuiteEnterprise) TestReviewComments() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	directChannelID := s.fakeMattermost.GetDirectChannelFor(s.fakeMattermost.GetBotUser(), s.reviewer1MattermostUser).ID

	s.startApp()

	// Test setup: we create an access request and wait for its message
	userName := integration.Requester1UserName
	req := s.CreateAccessRequest(ctx, userName, []string{s.reviewer1MattermostUser.Email})

	s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})

	post, err := s.fakeMattermost.CheckNewPost(ctx)
	require.NoError(t, err)
	assert.Equal(t, directChannelID, post.ChannelID)

	// Test execution: we submit a review and validate an update is posted
	err = s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	comment, err := s.fakeMattermost.CheckNewPost(ctx)
	require.NoError(t, err)
	assert.Equal(t, post.ChannelID, comment.ChannelID)
	assert.Equal(t, post.ID, comment.RootID)
	assert.Contains(t, comment.Message, integration.Reviewer1UserName+" reviewed the request", "comment must contain a review author")
	assert.Contains(t, comment.Message, "Resolution: ✅ APPROVED", "comment must contain a proposed state")
	assert.Contains(t, comment.Message, "Reason: ```\nokay```", "comment must contain a reason")

	// Test execution: we submit a second review and validate an update is posted
	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	comment, err = s.fakeMattermost.CheckNewPost(ctx)
	require.NoError(t, err)
	assert.Equal(t, post.ChannelID, comment.ChannelID)
	assert.Equal(t, post.ID, comment.RootID)
	assert.Contains(t, comment.Message, integration.Reviewer2UserName+" reviewed the request", "comment must contain a review author")
	assert.Contains(t, comment.Message, "Resolution: ❌ DENIED", "comment must contain a proposed state")
	assert.Contains(t, comment.Message, "Reason: ```\nnot okay```", "comment must contain a reason")
}

// TestApprovalByReview tests that the message is updated after the access request
// is reviewed and approved.
func (s *MattermostSuiteEnterprise) TestApprovalByReview() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its Discord message
	userName := integration.Requester1UserName
	req := s.CreateAccessRequest(ctx, userName, []string{s.reviewer1MattermostUser.Email})
	post, err := s.fakeMattermost.CheckNewPost(ctx)
	require.NoError(t, err, "no new messages posted")
	directChannelID := s.fakeMattermost.GetDirectChannelFor(s.fakeMattermost.GetBotUser(), s.reviewer1MattermostUser).ID
	assert.Equal(t, directChannelID, post.ChannelID)

	// Test execution: we submit a review and validate an update is posted
	err = s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	comment, err := s.fakeMattermost.CheckNewPost(ctx)
	require.NoError(t, err)
	assert.Equal(t, post.ChannelID, comment.ChannelID)
	assert.Equal(t, post.ID, comment.RootID)
	assert.Contains(t, comment.Message, integration.Reviewer1UserName+" reviewed the request", "comment must contain a review author")

	// Test execution: we submit a second review and validate an update is posted
	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "finally okay",
	})
	require.NoError(t, err)

	comment, err = s.fakeMattermost.CheckNewPost(ctx)
	require.NoError(t, err)
	assert.Equal(t, post.ChannelID, comment.ChannelID)
	assert.Equal(t, post.ID, comment.RootID)
	assert.Contains(t, comment.Message, integration.Reviewer2UserName+" reviewed the request", "comment must contain a review author")

	// When posting a review, the bot also updates the message to add the amount of reviewers.
	// This update is soon superseded by the "access allowed" update
	_, _ = s.fakeMattermost.CheckPostUpdate(ctx)

	// Finally, we validate the original message got updated to reflect the resolution status.
	postUpdate, err := s.fakeMattermost.CheckPostUpdate(ctx)
	require.NoError(t, err, "no messages updated")
	assert.Equal(t, post.ID, postUpdate.ID)
	assert.Equal(t, post.ChannelID, postUpdate.ChannelID)

	statusLine, err := parsePostField(postUpdate, "Status")
	require.NoError(t, err)
	assert.Equal(t, "✅ APPROVED", statusLine)

	matches := resolutionReasonRegexp.FindAllStringSubmatch(postUpdate.Message, -1)
	require.Len(t, matches, 1)
	require.Len(t, matches[0], 3)
	assert.Equal(t, "finally okay", matches[0][1])
	assert.Equal(t, "", matches[0][2])
}

// TestDenialByReview tests that the message is updated after the access request
// is reviewed and denied.
func (s *MattermostSuiteEnterprise) TestDenialByReview() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its Discord message
	userName := integration.Requester1UserName
	req := s.CreateAccessRequest(ctx, userName, []string{s.reviewer1MattermostUser.Email})
	post, err := s.fakeMattermost.CheckNewPost(ctx)
	require.NoError(t, err, "no new messages posted")
	directChannelID := s.fakeMattermost.GetDirectChannelFor(s.fakeMattermost.GetBotUser(), s.reviewer1MattermostUser).ID
	assert.Equal(t, directChannelID, post.ChannelID)

	// Test execution: we submit a review and validate an update is posted
	err = s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	comment, err := s.fakeMattermost.CheckNewPost(ctx)
	require.NoError(t, err)
	assert.Equal(t, post.ChannelID, comment.ChannelID)
	assert.Equal(t, post.ID, comment.RootID)
	assert.Contains(t, comment.Message, integration.Reviewer1UserName+" reviewed the request", "comment must contain a review author")

	// Test execution: we submit a second review and validate an update is posted
	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "finally not okay",
	})
	require.NoError(t, err)

	// When posting a review, the bot also updates the message to add the amount of reviewers.
	// This update is soon superseded by the "access allowed" update
	_, _ = s.fakeMattermost.CheckPostUpdate(ctx)

	// Finally, we validate the original message got updated to reflect the resolution status.
	comment, err = s.fakeMattermost.CheckNewPost(ctx)
	require.NoError(t, err)
	assert.Equal(t, post.ChannelID, comment.ChannelID)
	assert.Equal(t, post.ID, comment.RootID)
	assert.Contains(t, comment.Message, integration.Reviewer2UserName+" reviewed the request", "comment must contain a review author")

	postUpdate, err := s.fakeMattermost.CheckPostUpdate(ctx)
	require.NoError(t, err, "no messages updated")
	assert.Equal(t, post.ID, postUpdate.ID)
	assert.Equal(t, post.ChannelID, postUpdate.ChannelID)

	statusLine, err := parsePostField(postUpdate, "Status")
	require.NoError(t, err)
	assert.Equal(t, "❌ DENIED", statusLine)

	matches := resolutionReasonRegexp.FindAllStringSubmatch(postUpdate.Message, -1)
	require.Len(t, matches, 1)
	require.Len(t, matches[0], 3)
	assert.Equal(t, "finally not okay", matches[0][1])
	assert.Equal(t, "", matches[0][2])
}

// TestExpiration tests that when a request expires, its corresponding message
// is updated to reflect the new request state.
func (s *MattermostSuiteOSS) TestExpiration() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its Discord message
	userName := integration.RequesterOSSUserName
	req := s.CreateAccessRequest(ctx, userName, []string{s.reviewer1MattermostUser.Email})
	post, err := s.fakeMattermost.CheckNewPost(ctx)
	require.NoError(t, err, "no new messages posted")
	directChannelID := s.fakeMattermost.GetDirectChannelFor(s.fakeMattermost.GetBotUser(), s.reviewer1MattermostUser).ID
	assert.Equal(t, directChannelID, post.ChannelID)

	s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})

	// Test execution: we expire the request
	err = s.Ruler().DeleteAccessRequest(ctx, req.GetName()) // simulate expiration
	require.NoError(t, err)

	// Validating the plugin updated the Discord message to reflect that the request expired
	postUpdate, err := s.fakeMattermost.CheckPostUpdate(ctx)
	require.NoError(t, err, "no new messages updated")
	assert.Equal(t, post.ID, postUpdate.ID)
	assert.Equal(t, post.ChannelID, postUpdate.ChannelID)

	statusLine, err := parsePostField(postUpdate, "Status")
	require.NoError(t, err)
	assert.Equal(t, "⌛ EXPIRED", statusLine)
}

// TestRace validates that the plugin behaves properly and performs all the
// message updates when a lot of access requests are sent and reviewed in a very
// short time frame.
func (s *MattermostSuiteEnterprise) TestRace() {
	t := s.T()
	t.Skip("This test is flaky")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	err := logger.Setup(logger.Config{Severity: "info"}) // Turn off noisy debug logging
	require.NoError(t, err)

	botUser := s.fakeMattermost.GetBotUser()
	s.startApp()

	var (
		raceErr               error
		raceErrOnce           sync.Once
		postIDs               sync.Map
		postsCount            int32
		postUpdateCounters    sync.Map
		reviewCommentCounters sync.Map
	)
	setRaceErr := func(err error) error {
		raceErrOnce.Do(func() {
			raceErr = err
		})
		return err
	}

	process := lib.NewProcess(ctx)
	for i := 0; i < s.raceNumber; i++ {
		process.SpawnCritical(func(ctx context.Context) error {
			req, err := types.NewAccessRequest(uuid.New().String(), integration.Requester1UserName, "editor")
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			req.SetSuggestedReviewers([]string{s.reviewer1MattermostUser.Email, s.reviewer2MattermostUser.Email})
			if _, err := s.Requester1().CreateAccessRequestV2(ctx, req); err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			return nil
		})
	}

	// Having TWO suggested reviewers will post TWO messages for each request.
	// We also have approval threshold of TWO set in the role properties
	// so lets simply submit the approval from each of the suggested reviewers.
	//
	// Multiplier SIX means that we handle TWO messages for each request and also
	// TWO comments for each message: 2 * (1 message + 2 comments).
	for i := 0; i < 6*s.raceNumber; i++ {
		process.SpawnCritical(func(ctx context.Context) error {
			post, err := s.fakeMattermost.CheckNewPost(ctx)
			if err := trace.Wrap(err); err != nil {
				return setRaceErr(err)
			}

			if post.RootID == "" {
				// Handle "root" notifications.

				postKey := accessrequest.MessageData{ChannelID: post.ChannelID, MessageID: post.ID}
				if _, loaded := postIDs.LoadOrStore(postKey, struct{}{}); loaded {
					return setRaceErr(trace.Errorf("post %v already stored", postKey))
				}
				atomic.AddInt32(&postsCount, 1)

				reqID, err := parsePostField(post, "Request ID")
				if err != nil {
					return setRaceErr(trace.Wrap(err))
				}

				directChannel, ok := s.fakeMattermost.GetDirectChannel(post.ChannelID)
				if !ok {
					return setRaceErr(trace.Errorf("direct channel %s not found", post.ChannelID))
				}

				var userID string
				if directChannel.User2ID == botUser.ID {
					userID = directChannel.User1ID
				} else {
					userID = directChannel.User2ID
				}
				user, ok := s.fakeMattermost.GetUser(userID)
				if !ok {
					return setRaceErr(trace.Errorf("user %s not found", userID))
				}

				if err = s.ClientByName(user.Email).SubmitAccessRequestReview(ctx, reqID, types.AccessReview{
					Author:        user.Email,
					ProposedState: types.RequestState_APPROVED,
					Created:       time.Now(),
					Reason:        "okay",
				}); err != nil {
					return setRaceErr(trace.Wrap(err))
				}
			} else {
				// Handle review comments.

				postKey := accessrequest.MessageData{ChannelID: post.ChannelID, MessageID: post.RootID}
				var newCounter int32
				val, _ := reviewCommentCounters.LoadOrStore(postKey, &newCounter)
				counterPtr := val.(*int32)
				atomic.AddInt32(counterPtr, 1)
			}

			return nil
		})
	}

	// Multiplier TWO means that we handle updates for each of the two messages posted to reviewers.
	for i := 0; i < 2*2*s.raceNumber; i++ {
		process.SpawnCritical(func(ctx context.Context) error {
			post, err := s.fakeMattermost.CheckPostUpdate(ctx)
			if err != nil {
				return setRaceErr(trace.Wrap(err, "error from post update consumer"))
			}

			postKey := accessrequest.MessageData{ChannelID: post.ChannelID, MessageID: post.ID}
			var newCounter int32
			val, _ := postUpdateCounters.LoadOrStore(postKey, &newCounter)
			counterPtr := val.(*int32)
			atomic.AddInt32(counterPtr, 1)

			return nil
		})
	}

	process.Terminate()
	<-process.Done()
	require.NoError(t, raceErr)

	assert.Equal(t, int32(2*s.raceNumber), postsCount)
	postIDs.Range(func(key, value interface{}) bool {
		next := true

		val, loaded := reviewCommentCounters.LoadAndDelete(key)
		next = next && assert.True(t, loaded)
		counterPtr := val.(*int32)
		next = next && assert.Equal(t, int32(2), *counterPtr)

		val, loaded = postUpdateCounters.LoadAndDelete(key)
		next = next && assert.True(t, loaded)
		counterPtr = val.(*int32)
		// Each message should be updated 2 times
		next = next && assert.Equal(t, int32(2), *counterPtr)

		return next
	})
}

func (s *MattermostSuiteOSS) TestRecipientsConfig() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	directChannel1 := s.fakeMattermost.GetDirectChannelFor(s.fakeMattermost.GetBotUser(), s.reviewer1MattermostUser)

	team := s.fakeMattermost.StoreTeam(mattermost.Team{Name: "team-llama"})
	channel2 := s.fakeMattermost.StoreChannel(mattermost.Channel{Name: "channel-llama", TeamID: team.ID})

	// Test an email and a team/channel
	s.appConfig.Recipients = common.RawRecipientsMap{
		types.Wildcard: []string{"team-llama/channel-llama", s.reviewer1MattermostUser.Email},
	}

	s.startApp()

	userName := integration.RequesterOSSUserName
	request := s.CreateAccessRequest(ctx, userName, nil)
	pluginData := s.checkPluginData(ctx, request.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})
	assert.Len(t, pluginData.SentMessages, 2)

	var (
		msg      mattermost.Post
		messages []mattermost.Post
	)

	messageSet := make(MattermostDataPostSet)

	msg, err := s.fakeMattermost.CheckNewPost(ctx)
	require.NoError(t, err)
	messageSet.Add(accessrequest.MessageData{ChannelID: msg.ChannelID, MessageID: msg.ID})
	messages = append(messages, msg)

	msg, err = s.fakeMattermost.CheckNewPost(ctx)
	require.NoError(t, err)
	messageSet.Add(accessrequest.MessageData{ChannelID: msg.ChannelID, MessageID: msg.ID})
	messages = append(messages, msg)

	assert.Len(t, messageSet, 2)
	assert.Contains(t, messageSet, pluginData.SentMessages[0])
	assert.Contains(t, messageSet, pluginData.SentMessages[1])

	sort.Sort(MattermostPostSlice(messages))

	assert.Equal(t, directChannel1.ID, messages[0].ChannelID)
	assert.Equal(t, channel2.ID, messages[1].ChannelID)
}
