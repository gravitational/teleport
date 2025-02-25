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
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	v1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/integrations/access/accessrequest"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/auth"
	"github.com/gravitational/teleport/integrations/access/slack"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
)

// SlackBaseSuite is the Slack access plugin test suite.
// It implements the testify.TestingSuite interface.
type SlackBaseSuite struct {
	*integration.AccessRequestSuite
	appConfig             *slack.Config
	raceNumber            int
	fakeSlack             *FakeSlack
	fakeStatusSink        *integration.FakeStatusSink
	requester1SlackUser   slack.User
	requesterOSSSlackUser slack.User
	reviewer1SlackUser    slack.User
	reviewer2SlackUser    slack.User
}

// SetupTest starts a fake Slack, generates the plugin configuration, and loads
// the fixtures in Slack. It runs for each test.
func (s *SlackBaseSuite) SetupTest() {
	t := s.T()

	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)
	s.raceNumber = runtime.GOMAXPROCS(0)

	s.fakeSlack = NewFakeSlack(slack.User{Name: "slackbot"}, s.raceNumber)
	t.Cleanup(s.fakeSlack.Close)

	// We need requester users as well, the slack plugin sends messages to users
	// when their access request got approved.
	s.requesterOSSSlackUser = s.fakeSlack.StoreUser(slack.User{Name: "Requester OSS", Profile: slack.UserProfile{Email: integration.RequesterOSSUserName}})
	s.requester1SlackUser = s.fakeSlack.StoreUser(slack.User{Name: "Vladimir", Profile: slack.UserProfile{Email: integration.Requester1UserName}})
	s.reviewer1SlackUser = s.fakeSlack.StoreUser(slack.User{Profile: slack.UserProfile{Email: integration.Reviewer1UserName}})
	s.reviewer2SlackUser = s.fakeSlack.StoreUser(slack.User{Profile: slack.UserProfile{Email: integration.Reviewer2UserName}})

	s.fakeStatusSink = &integration.FakeStatusSink{}

	var conf slack.Config
	conf.Teleport = s.TeleportConfig()
	conf.Slack.Token = "000000"
	conf.Slack.APIURL = s.fakeSlack.URL() + "/"
	conf.AccessTokenProvider = auth.NewStaticAccessTokenProvider(conf.Slack.Token)
	conf.StatusSink = s.fakeStatusSink
	conf.PluginType = types.PluginTypeSlack

	s.appConfig = &conf
}

// startApp starts the Slack plugin, waits for it to become ready and returns.
func (s *SlackBaseSuite) startApp() {
	t := s.T()
	t.Helper()

	app := slack.NewSlackApp(s.appConfig)
	integration.RunAndWaitReady(t, app)
}

// SlackSuiteOSS contains all tests that support running against a Teleport
// OSS Server.
type SlackSuiteOSS struct {
	SlackBaseSuite
}

// SlackSuiteEnterprise contains all tests that require a Teleport Enterprise
// to run.
type SlackSuiteEnterprise struct {
	SlackBaseSuite
}

// SetupTest overrides SlackBaseSuite.SetupTest to check the Teleport features
// before each test.
func (s *SlackSuiteEnterprise) SetupTest() {
	t := s.T()
	s.RequireAdvancedWorkflow(t)
	s.SlackBaseSuite.SetupTest()
}

// TestMessagePosting validates that a message is sent to each recipient
// specified in the suggested reviewers. It also checks that the message
// content is correct.
func (s *SlackSuiteOSS) TestMessagePosting() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// max size of request was decreased here: https://github.com/gravitational/teleport/pull/13298
	s.SetReasonPadding(4000)

	s.startApp()

	const numMessages = 3
	userName := integration.RequesterOSSUserName

	// Test execution: we create an access request and specify 2 recipients as
	// suggested reviewers.
	request := s.CreateAccessRequest(ctx, userName, []string{
		s.reviewer1SlackUser.Profile.Email,
		s.reviewer2SlackUser.Profile.Email,
	})

	pluginData := s.checkPluginData(ctx, request.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})
	assert.Len(t, pluginData.SentMessages, numMessages)

	// We validate we got 3 messages: one for each recipient and one for the requester.
	var messages []slack.Message
	messageSet := make(SlackDataMessageSet)
	for i := 0; i < numMessages; i++ {
		msg, err := s.fakeSlack.CheckNewMessage(ctx)
		require.NoError(t, err)
		messageSet.Add(accessrequest.MessageData{ChannelID: msg.Channel, MessageID: msg.Timestamp})
		messages = append(messages, msg)
	}

	assert.Len(t, messageSet, numMessages)
	for i := 0; i < numMessages; i++ {
		assert.Contains(t, messageSet, pluginData.SentMessages[i])
	}

	// Validate the message content
	sort.Sort(SlackMessageSlice(messages))

	assert.Equal(t, s.requesterOSSSlackUser.ID, messages[0].Channel)
	assert.Equal(t, s.reviewer1SlackUser.ID, messages[1].Channel)
	assert.Equal(t, s.reviewer2SlackUser.ID, messages[2].Channel)

	msgUser, err := parseMessageField(messages[0], "User")
	require.NoError(t, err)
	assert.Equal(t, userName, msgUser)

	block, ok := messages[0].BlockItems[1].Block.(slack.SectionBlock)
	require.True(t, ok)
	t.Logf("%q", block.Text.GetText())
	matches := requestReasonRegexp.FindAllStringSubmatch(block.Text.GetText(), -1)
	require.Len(t, matches, 1)
	require.Len(t, matches[0], 3)
	assert.Equal(t, "because of "+strings.Repeat("A", 489), matches[0][1])
	assert.Equal(t, " (truncated)", matches[0][2])

	statusLine, err := getStatusLine(messages[0])
	require.NoError(t, err)
	assert.Equal(t, "*Status*: ⏳ PENDING", statusLine)

	assert.Equal(t, types.PluginStatusCode_RUNNING, s.fakeStatusSink.Get().GetCode())
}

// TestRecipientsConfig checks that the recipient configuration accepts both
// referencing users by their email, or their slack user ID.
func (s *SlackSuiteOSS) TestRecipientsConfig() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// Test setup: We define a recipient by email, and a recipient by user id.
	s.appConfig.Recipients = common.RawRecipientsMap{
		types.Wildcard: []string{
			s.reviewer2SlackUser.Profile.Email,
			s.reviewer1SlackUser.ID,
		},
	}

	s.startApp()
	const numMessages = 3

	// Test execution: we create an access request
	userName := integration.RequesterOSSUserName
	request := s.CreateAccessRequest(ctx, userName, nil)
	pluginData := s.checkPluginData(ctx, request.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})
	assert.Len(t, pluginData.SentMessages, numMessages)

	var messages []slack.Message

	messageSet := make(SlackDataMessageSet)

	// Validate we got 3 messages: one for each recipient and one for the requester.
	for i := 0; i < numMessages; i++ {
		msg, err := s.fakeSlack.CheckNewMessage(ctx)
		require.NoError(t, err)
		messageSet.Add(accessrequest.MessageData{ChannelID: msg.Channel, MessageID: msg.Timestamp})
		messages = append(messages, msg)
	}

	assert.Len(t, messageSet, numMessages)
	for i := 0; i < numMessages; i++ {
		assert.Contains(t, messageSet, pluginData.SentMessages[i])
	}

	// Validate the message recipients
	sort.Sort(SlackMessageSlice(messages))
	assert.Equal(t, s.requesterOSSSlackUser.ID, messages[0].Channel)
	assert.Equal(t, s.reviewer1SlackUser.ID, messages[1].Channel)
	assert.Equal(t, s.reviewer2SlackUser.ID, messages[2].Channel)
}

func (s *SlackSuiteOSS) TestRecipientsFromAccessMonitoringRule() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// Setup base config to ensure access monitoring rule recipient take precidence
	s.appConfig.Recipients = common.RawRecipientsMap{
		types.Wildcard: []string{
			s.reviewer2SlackUser.Profile.Email,
		},
	}

	s.startApp()
	const numMessages = 3

	_, err := s.ClientByName(integration.RulerUserName).
		AccessMonitoringRulesClient().
		CreateAccessMonitoringRule(ctx, &accessmonitoringrulesv1.AccessMonitoringRule{
			Kind:    types.KindAccessMonitoringRule,
			Version: types.V1,
			Metadata: &v1.Metadata{
				Name: "test-slack-amr",
			},
			Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
				Subjects:  []string{types.KindAccessRequest},
				Condition: "!is_empty(access_request.spec.roles)",
				Notification: &accessmonitoringrulesv1.Notification{
					Name: "slack",
					Recipients: []string{
						s.reviewer1SlackUser.ID,
						s.reviewer2SlackUser.Profile.Email,
					},
				},
			},
		})
	assert.NoError(t, err)

	// Test execution: we create an access request
	userName := integration.RequesterOSSUserName
	request := s.CreateAccessRequest(ctx, userName, nil)
	pluginData := s.checkPluginData(ctx, request.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})
	assert.Len(t, pluginData.SentMessages, numMessages)

	var messages []slack.Message

	messageSet := make(SlackDataMessageSet)

	// Validate we got 3 messages: one for each recipient and one for the requester.
	for i := 0; i < numMessages; i++ {
		msg, err := s.fakeSlack.CheckNewMessage(ctx)
		require.NoError(t, err)
		messageSet.Add(accessrequest.MessageData{ChannelID: msg.Channel, MessageID: msg.Timestamp})
		messages = append(messages, msg)
	}

	assert.Len(t, messageSet, numMessages)
	for i := 0; i < numMessages; i++ {
		assert.Contains(t, messageSet, pluginData.SentMessages[i])
	}

	// Validate the message recipients
	sort.Sort(SlackMessageSlice(messages))
	assert.Equal(t, s.requesterOSSSlackUser.ID, messages[0].Channel)
	assert.Equal(t, s.reviewer1SlackUser.ID, messages[1].Channel)
	assert.Equal(t, s.reviewer2SlackUser.ID, messages[2].Channel)

	assert.NoError(t, s.ClientByName(integration.RulerUserName).
		AccessMonitoringRulesClient().DeleteAccessMonitoringRule(ctx, "test-slack-amr"))
}

func (s *SlackSuiteOSS) TestRecipientsFromAccessMonitoringRuleAfterUpdate() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// Setup base config to ensure access monitoring rule recipient take precidence
	s.appConfig.Recipients = common.RawRecipientsMap{
		types.Wildcard: []string{
			s.reviewer2SlackUser.Profile.Email,
		},
	}

	s.startApp()
	const numMessagesInitial = 3
	const numMessagesFinal = 2

	_, err := s.ClientByName(integration.RulerUserName).
		AccessMonitoringRulesClient().
		CreateAccessMonitoringRule(ctx, &accessmonitoringrulesv1.AccessMonitoringRule{
			Kind:    types.KindAccessMonitoringRule,
			Version: types.V1,
			Metadata: &v1.Metadata{
				Name: "test-slack-amr-2",
			},
			Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
				Subjects:  []string{types.KindAccessRequest},
				Condition: "!is_empty(access_request.spec.roles)",
				Notification: &accessmonitoringrulesv1.Notification{
					Name: "slack",
					Recipients: []string{
						s.reviewer1SlackUser.ID,
						s.reviewer2SlackUser.Profile.Email,
					},
				},
			},
		})
	assert.NoError(t, err)

	// Test execution: we create an access request
	userName := integration.RequesterOSSUserName
	request := s.CreateAccessRequest(ctx, userName, nil)
	pluginData := s.checkPluginData(ctx, request.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})
	assert.Len(t, pluginData.SentMessages, numMessagesInitial)

	var messages []slack.Message

	messageSet := make(SlackDataMessageSet)

	// Validate we got 3 messages: one for each recipient and one for the requester.
	for i := 0; i < numMessagesInitial; i++ {
		msg, err := s.fakeSlack.CheckNewMessage(ctx)
		require.NoError(t, err)
		messageSet.Add(accessrequest.MessageData{ChannelID: msg.Channel, MessageID: msg.Timestamp})
		messages = append(messages, msg)
	}

	assert.Len(t, messageSet, numMessagesInitial)
	for i := 0; i < numMessagesInitial; i++ {
		assert.Contains(t, messageSet, pluginData.SentMessages[i])
	}

	// Validate the message recipients
	sort.Sort(SlackMessageSlice(messages))
	assert.Equal(t, s.requesterOSSSlackUser.ID, messages[0].Channel)
	assert.Equal(t, s.reviewer1SlackUser.ID, messages[1].Channel)
	assert.Equal(t, s.reviewer2SlackUser.ID, messages[2].Channel)

	_, err = s.ClientByName(integration.RulerUserName).
		AccessMonitoringRulesClient().
		UpdateAccessMonitoringRule(ctx, &accessmonitoringrulesv1.AccessMonitoringRule{
			Kind:    types.KindAccessMonitoringRule,
			Version: types.V1,
			Metadata: &v1.Metadata{
				Name: "test-slack-amr-2",
			},
			Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
				Subjects:  []string{"someOtherKind"},
				Condition: "!is_empty(access_request.spec.roles)",
				Notification: &accessmonitoringrulesv1.Notification{
					Name: "slack",
					Recipients: []string{
						s.reviewer1SlackUser.ID,
						s.reviewer2SlackUser.Profile.Email,
					},
				},
			},
		})
	assert.NoError(t, err)

	messages = []slack.Message{}
	messageSet = make(SlackDataMessageSet)

	// Test execution: we create an access request
	request = s.CreateAccessRequest(ctx, userName, nil)
	pluginData = s.checkPluginData(ctx, request.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})
	assert.Len(t, pluginData.SentMessages, numMessagesFinal)

	// Validate we got 2 messages since the base config should kick back in
	for i := 0; i < numMessagesFinal; i++ {
		msg, err := s.fakeSlack.CheckNewMessage(ctx)
		require.NoError(t, err)
		messageSet.Add(accessrequest.MessageData{ChannelID: msg.Channel, MessageID: msg.Timestamp})
		messages = append(messages, msg)
	}

	assert.Len(t, messageSet, numMessagesFinal)
	for i := numMessagesInitial - 1; i < numMessagesFinal; i++ {
		assert.Contains(t, messageSet, pluginData.SentMessages[i])
	}

	// Validate the message recipients
	sort.Sort(SlackMessageSlice(messages))
	assert.Equal(t, s.requesterOSSSlackUser.ID, messages[0].Channel)
	assert.Equal(t, s.reviewer2SlackUser.ID, messages[1].Channel)

	assert.NoError(t, s.ClientByName(integration.RulerUserName).
		AccessMonitoringRulesClient().DeleteAccessMonitoringRule(ctx, "test-slack-amr-2"))
}

// TestApproval tests that when a request is approved, its corresponding message
// is updated to reflect the new request state.
func (s *SlackSuiteOSS) TestApproval() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its Slack message
	userName := integration.RequesterOSSUserName
	req := s.CreateAccessRequest(ctx, userName, []string{s.reviewer1SlackUser.Profile.Email})
	msgs := s.checkNewMessages(t, ctx, channelsToMessages(s.requesterOSSSlackUser.ID, s.reviewer1SlackUser.ID), matchOnlyOnChannel)

	// Test execution: we approve the request
	err := s.Ruler().ApproveAccessRequest(ctx, req.GetName(), "okay")
	require.NoError(t, err)

	// Validating the plugin updated the message to reflect that it got approved
	s.checkNewMessageUpdateByAPI(t, ctx, msgs, matchByTimestamp, func(t *testing.T, msgUpdate slack.Message) {
		statusLine, err := getStatusLine(msgUpdate)
		require.NoError(t, err)
		assert.Equal(t, "*Status*: ✅ APPROVED\n*Resolution reason*: ```\nokay```", statusLine)
	})

	s.checkNewMessages(t, ctx, channelsToMessages(s.requesterOSSSlackUser.ID), matchOnlyOnChannel, func(t *testing.T, m slack.Message) {
		line := fmt.Sprintf("Request with ID %q has been updated: *%s*", req.GetName(), types.RequestState_APPROVED.String())
		assert.Equal(t, line, m.BlockItems[0].Block.(slack.SectionBlock).Text.GetText())
	})

}

// TestDenial tests that when a request is denied, its corresponding message
// is updated to reflect the new request state.
func (s *SlackSuiteOSS) TestDenial() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its Discord message
	userName := integration.RequesterOSSUserName
	req := s.CreateAccessRequest(ctx, userName, []string{s.reviewer1SlackUser.Profile.Email})
	msgs := s.checkNewMessages(t, ctx, channelsToMessages(s.requesterOSSSlackUser.ID, s.reviewer1SlackUser.ID), matchOnlyOnChannel)

	// Test execution: we deny the request
	// max size of request was decreased here: https://github.com/gravitational/teleport/pull/13298
	err := s.Ruler().DenyAccessRequest(ctx, req.GetName(), "not okay "+strings.Repeat("A", 4000))
	require.NoError(t, err)

	// Validating the plugin updated the message to reflect that it got denied
	s.checkNewMessageUpdateByAPI(t, ctx, msgs, matchByTimestamp, func(t *testing.T, msgUpdate slack.Message) {
		statusLine, err := getStatusLine(msgUpdate)
		require.NoError(t, err)
		assert.Equal(t, "*Status*: ❌ DENIED\n*Resolution reason*: ```\nnot okay "+strings.Repeat("A", 491)+"``` (truncated)", statusLine)
	})
}

// TestReviewReplies tests that a reply is sent after the access request
// is reviewed. Each review should be reflected in the original message.
func (s *SlackSuiteEnterprise) TestReviewReplies() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its Slack messages
	userName := integration.Requester1UserName
	req := s.CreateAccessRequest(ctx, userName, []string{s.reviewer1SlackUser.Profile.Email})
	s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})

	msgs := s.checkNewMessages(t, ctx, channelsToMessages(s.requester1SlackUser.ID, s.reviewer1SlackUser.ID), matchOnlyOnChannel)

	// Test execution: we submit a review and validate a reply was sent.
	err := s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	s.checkNewMessages(t, ctx, msgs, matchByThreadTs, func(t *testing.T, reply slack.Message) {
		assert.Contains(t, reply.Text, integration.Reviewer1UserName+" reviewed the request", "reply must contain a review author")
		assert.Contains(t, reply.Text, "Resolution: ✅ APPROVED", "reply must contain a proposed state")
		assert.Contains(t, reply.Text, "Reason: ```\nokay```", "reply must contain a reason")
	})

	// Test execution: we submit a second review and validate a reply was sent.
	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	s.checkNewMessages(t, ctx, msgs, matchByThreadTs, func(t *testing.T, reply slack.Message) {
		assert.Contains(t, reply.Text, integration.Reviewer2UserName+" reviewed the request", "reply must contain a review author")
		assert.Contains(t, reply.Text, "Resolution: ❌ DENIED", "reply must contain a proposed state")
		assert.Contains(t, reply.Text, "Reason: ```\nnot okay```", "reply must contain a reason")
	})
}

// TestApprovalByReview tests that the message is updated after the access request
// is reviewed and approved.
func (s *SlackSuiteEnterprise) TestApprovalByReview() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its messages
	userName := integration.Requester1UserName
	req := s.CreateAccessRequest(ctx, userName, []string{s.reviewer1SlackUser.Profile.Email})
	msgs := s.checkNewMessages(t, ctx, channelsToMessages(s.requester1SlackUser.ID, s.reviewer1SlackUser.ID), matchOnlyOnChannel)

	// Test execution: we submit a review and validate a reply was sent.
	err := s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	s.checkNewMessages(t, ctx, msgs, matchByThreadTs, func(t *testing.T, reply slack.Message) {
		assert.Contains(t, reply.Text, integration.Reviewer1UserName+" reviewed the request", "reply must contain a review author")
	})

	s.checkNewMessageUpdateByAPI(t, ctx, msgs, matchByTimestamp, func(t *testing.T, msgUpdate slack.Message) {
		statusLine, err := getStatusLine(msgUpdate)
		require.NoError(t, err)
		assert.Equal(t, "*Status*: ⏳ PENDING", statusLine)
	})

	// Test execution: we submit a second review and validate a reply was sent.
	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "finally okay",
	})
	require.NoError(t, err)

	s.checkNewMessages(t, ctx, msgs, matchByThreadTs, func(t *testing.T, reply slack.Message) {
		assert.Contains(t, reply.Text, integration.Reviewer2UserName+" reviewed the request", "reply must contain a review author")
	})

	// Validating the plugin updated the message to reflect that the request got approved.
	s.checkNewMessageUpdateByAPI(t, ctx, msgs, matchByTimestamp, func(t *testing.T, msgUpdate slack.Message) {
		statusLine, err := getStatusLine(msgUpdate)
		require.NoError(t, err)
		assert.Equal(t, "*Status*: ✅ APPROVED\n*Resolution reason*: ```\nfinally okay```", statusLine)
	})
}

// TestDenialByReview tests that the message is updated after the access request
// is reviewed and denied.
func (s *SlackSuiteEnterprise) TestDenialByReview() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its messages.
	userName := integration.Requester1UserName
	req := s.CreateAccessRequest(ctx, userName, []string{s.reviewer1SlackUser.Profile.Email})
	msgs := s.checkNewMessages(t, ctx, channelsToMessages(s.requester1SlackUser.ID, s.reviewer1SlackUser.ID), matchOnlyOnChannel)

	// Test execution: we submit a review and validate a reply was sent.
	err := s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	s.checkNewMessages(t, ctx, msgs, matchByThreadTs, func(t *testing.T, reply slack.Message) {
		assert.Contains(t, reply.Text, integration.Reviewer1UserName+" reviewed the request", "reply must contain a review author")
	})

	s.checkNewMessageUpdateByAPI(t, ctx, msgs, matchByTimestamp, func(t *testing.T, msgUpdate slack.Message) {
		statusLine, err := getStatusLine(msgUpdate)
		require.NoError(t, err)
		assert.Equal(t, "*Status*: ⏳ PENDING", statusLine)
	})

	// Test execution: we submit a second review and validate a reply was sent.
	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "finally not okay",
	})
	require.NoError(t, err)

	s.checkNewMessages(t, ctx, msgs, matchByThreadTs, func(t *testing.T, reply slack.Message) {
		assert.Contains(t, reply.Text, integration.Reviewer2UserName+" reviewed the request", "reply must contain a review author")
	})

	// Validating the plugin updated the message to reflect that the request got denied.
	s.checkNewMessageUpdateByAPI(t, ctx, msgs, matchByTimestamp, func(t *testing.T, msgUpdate slack.Message) {
		statusLine, err := getStatusLine(msgUpdate)
		require.NoError(t, err)
		assert.Equal(t, "*Status*: ❌ DENIED\n*Resolution reason*: ```\nfinally not okay```", statusLine)
	})
}

// TestExpiration tests that when a request expires, its corresponding message
// is updated to reflect the new request state.
func (s *SlackSuiteOSS) TestExpiration() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	// Test setup: we create an access request and wait for its message.
	userName := integration.RequesterOSSUserName
	req := s.CreateAccessRequest(ctx, userName, []string{s.reviewer1SlackUser.Profile.Email})
	msgs := s.checkNewMessages(t, ctx, channelsToMessages(s.requesterOSSSlackUser.ID, s.reviewer1SlackUser.ID), matchOnlyOnChannel)

	s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})

	// Test execution: we expire the request
	err := s.Ruler().DeleteAccessRequest(ctx, req.GetName()) // simulate expiration
	require.NoError(t, err)

	// Validating the plugin updated the message to reflect that the request expired.
	s.checkNewMessageUpdateByAPI(t, ctx, msgs, matchByTimestamp, func(t *testing.T, msgUpdate slack.Message) {
		statusLine, err := getStatusLine(msgUpdate)
		require.NoError(t, err)
		assert.Equal(t, "*Status*: ⌛ EXPIRED", statusLine)
	})
}

// TestRace validates that the plugin behaves properly and performs all the
// message updates when a lot of access requests are sent and reviewed in a very
// short time frame.
func (s *SlackSuiteEnterprise) TestRace() {
	t := s.T()
	t.Skip("This test is flaky")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	err := logger.Setup(logger.Config{Severity: "info"}) // Turn off noisy debug logging
	require.NoError(t, err)

	s.startApp()

	var (
		raceErr             error
		raceErrOnce         sync.Once
		threadMsgIDs        sync.Map
		threadMsgsCount     int32
		msgUpdateCounters   sync.Map
		reviewReplyCounters sync.Map
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
			req.SetSuggestedReviewers([]string{s.reviewer1SlackUser.Profile.Email, s.reviewer2SlackUser.Profile.Email})
			if _, err := s.Requester1().CreateAccessRequestV2(ctx, req); err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			return nil
		})
	}

	// Having TWO suggested reviewers will post THREE messages for each request (including the requestor).
	// We also have approval threshold of TWO set in the role properties
	// so lets simply submit the approval from each of the suggested reviewers.
	//
	// Multiplier NINE means that we handle THREE messages for each request and also
	// TWO comments for each message: 2 * (1 message + 2 comments).
	for i := 0; i < 9*s.raceNumber; i++ {
		process.SpawnCritical(func(ctx context.Context) error {
			msg, err := s.fakeSlack.CheckNewMessage(ctx)
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}

			if msg.ThreadTs == "" {
				// Handle "root" notifications.

				threadMsgKey := accessrequest.MessageData{ChannelID: msg.Channel, MessageID: msg.Timestamp}
				if _, loaded := threadMsgIDs.LoadOrStore(threadMsgKey, struct{}{}); loaded {
					return setRaceErr(trace.Errorf("thread %v already stored", threadMsgKey))
				}
				atomic.AddInt32(&threadMsgsCount, 1)

				user, ok := s.fakeSlack.GetUser(msg.Channel)
				if !ok {
					return setRaceErr(trace.Errorf("user %s is not found", msg.Channel))
				}

				reqID, err := parseMessageField(msg, "ID")
				if err != nil {
					return setRaceErr(trace.Wrap(err))
				}

				// The requestor can't submit reviews.
				if user.ID == s.requester1SlackUser.ID {
					return nil
				}

				if err = s.ClientByName(user.Profile.Email).SubmitAccessRequestReview(ctx, reqID, types.AccessReview{
					Author:        user.Profile.Email,
					ProposedState: types.RequestState_APPROVED,
					Created:       time.Now(),
					Reason:        "okay",
				}); err != nil {
					return setRaceErr(trace.Wrap(err))
				}
			} else {
				// Handle review comments.

				threadMsgKey := accessrequest.MessageData{ChannelID: msg.Channel, MessageID: msg.ThreadTs}
				var newCounter int32
				val, _ := reviewReplyCounters.LoadOrStore(threadMsgKey, &newCounter)
				counterPtr := val.(*int32)
				atomic.AddInt32(counterPtr, 1)
			}

			return nil
		})
	}

	// Multiplier THREE means that we handle the 2 updates for each of the two messages posted to reviewers.
	for i := 0; i < 3*2*s.raceNumber; i++ {
		process.SpawnCritical(func(ctx context.Context) error {
			msg, err := s.fakeSlack.CheckMessageUpdateByAPI(ctx)
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}

			threadMsgKey := accessrequest.MessageData{ChannelID: msg.Channel, MessageID: msg.Timestamp}
			var newCounter int32
			val, _ := msgUpdateCounters.LoadOrStore(threadMsgKey, &newCounter)
			counterPtr := val.(*int32)
			atomic.AddInt32(counterPtr, 1)

			return nil
		})
	}

	process.Terminate()
	<-process.Done()
	require.NoError(t, raceErr)

	assert.Equal(t, int32(3*s.raceNumber), threadMsgsCount)
	threadMsgIDs.Range(func(key, value interface{}) bool {
		next := true

		val, loaded := reviewReplyCounters.LoadAndDelete(key)
		next = next && assert.True(t, loaded)
		counterPtr := val.(*int32)
		next = next && assert.Equal(t, int32(2), *counterPtr)

		val, loaded = msgUpdateCounters.LoadAndDelete(key)
		next = next && assert.True(t, loaded)
		counterPtr = val.(*int32)
		// Each message should be updated 2 times
		next = next && assert.Equal(t, int32(2), *counterPtr)

		return next
	})
}

// TestAccessListReminder validates that Access List reminders are sent before
// the Access List expires.
func (s *SlackSuiteEnterprise) TestAccessListReminder_Singular() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	clock := clockwork.NewFakeClockAt(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))
	s.appConfig.Clock = clock
	s.startApp()

	// Test setup: create an access list
	accessList, err := accesslist.NewAccessList(header.Metadata{
		Name: "access-list",
	}, accesslist.Spec{
		Title: "simple title",
		Grants: accesslist.Grants{
			Roles: []string{"grant"},
		},
		Owners: []accesslist.Owner{
			{Name: integration.Reviewer1UserName},
		},
		Audit: accesslist.Audit{
			NextAuditDate: time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC),
		},
	})
	require.NoError(t, err)

	_, err = s.Ruler().AccessListClient().UpsertAccessList(ctx, accessList)
	require.NoError(t, err)

	// Test execution: move the clock to 2 weeks before expiry
	// This should trigger a reminder.
	clock.BlockUntil(1)
	clock.Advance(45 * 24 * time.Hour)
	s.requireReminderMsgEqual(ctx, s.reviewer1SlackUser.ID, "Access List *simple title* is due for a review by 2023-03-01. Please review it soon!")

	// Test execution: move the clock to 1 week before expiry
	// This should trigger a reminder.
	clock.BlockUntil(1)
	clock.Advance(7 * 24 * time.Hour)
	s.requireReminderMsgEqual(ctx, s.reviewer1SlackUser.ID, "Access List *simple title* is due for a review by 2023-03-01. Please review it soon!")

	// Test execution: move the clock to the expiry day
	// This should trigger a reminder.
	clock.BlockUntil(1)
	clock.Advance(7 * 24 * time.Hour)
	s.requireReminderMsgEqual(ctx, s.reviewer1SlackUser.ID, "Access List *simple title* is due for a review by 2023-03-01. Please review it soon!")

	// Test execution: move the clock after the expiry day
	// This should trigger a reminder.
	clock.BlockUntil(1)
	clock.Advance(7 * 24 * time.Hour)
	s.requireReminderMsgEqual(ctx, s.reviewer1SlackUser.ID, "Access List *simple title* is 7 day(s) past due for a review! Please review it.")
}

// TestAccessListReminder_Batched validates that Access List reminders are sent in batches
// if multiple access lists are given.
func (s *SlackSuiteEnterprise) TestAccessListReminder_Batched() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	clock := clockwork.NewFakeClockAt(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))
	s.appConfig.Clock = clock
	s.startApp()

	// Test setup: create a couple accesslists

	accessList1, err := accesslist.NewAccessList(header.Metadata{
		Name: "access-list1",
	}, accesslist.Spec{
		Title: "simple title one",
		Grants: accesslist.Grants{
			Roles: []string{"grant"},
		},
		Owners: []accesslist.Owner{
			{Name: integration.Reviewer1UserName},
		},
		Audit: accesslist.Audit{
			NextAuditDate: time.Date(2023, 3, 2, 0, 0, 0, 0, time.UTC),
		},
	})
	require.NoError(t, err)
	_, err = s.Ruler().AccessListClient().UpsertAccessList(ctx, accessList1)
	require.NoError(t, err)

	accessList2, err := accesslist.NewAccessList(header.Metadata{
		Name: "access-list2",
	}, accesslist.Spec{
		Title: "simple title two",
		Grants: accesslist.Grants{
			Roles: []string{"grant"},
		},
		Owners: []accesslist.Owner{
			{Name: integration.Reviewer1UserName},
		},
		Audit: accesslist.Audit{
			NextAuditDate: time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC),
		},
	})
	require.NoError(t, err)
	_, err = s.Ruler().AccessListClient().UpsertAccessList(ctx, accessList2)
	require.NoError(t, err)

	// Trigger a reminder.
	clock.BlockUntil(1)
	clock.Advance(46 * 25 * time.Hour)
	s.requireReminderMsgEqual(ctx, s.reviewer1SlackUser.ID, "2 Access Lists are due for reviews, earliest of which is due by 2023-03-01")

	// Make it overdue.
	clock.BlockUntil(1)
	clock.Advance(20 * 24 * time.Hour)
	s.requireReminderMsgEqual(ctx, s.reviewer1SlackUser.ID, "2 Access Lists are due for reviews, earliest of which is 8 day(s) past due")
}

func (s *SlackBaseSuite) requireReminderMsgEqual(ctx context.Context, id, text string) {
	s.T().Helper()
	t := s.T()

	msg, err := s.fakeSlack.CheckNewMessage(ctx)
	require.NoError(t, err)
	require.Equal(t, id, msg.Channel)
	require.IsType(t, slack.SectionBlock{}, msg.BlockItems[0].Block)
	require.Contains(t, (msg.BlockItems[0].Block).(slack.SectionBlock).Text.GetText(), text)
}
