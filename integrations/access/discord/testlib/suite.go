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

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	v1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/accessrequest"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/discord"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
)

var msgFieldRegexp = regexp.MustCompile(`(?im)^\*([a-zA-Z ]+)\*: (.+)$`)
var requestReasonRegexp = regexp.MustCompile("(?im)^\\*Reason\\*:\\ ```\\n(.*?)```(.*?)$")

// DiscordBaseSuite is the discord access plugin test suite.
// It implements the testify.TestingSuite interface.
type DiscordBaseSuite struct {
	*integration.AccessRequestSuite
	appConfig   *discord.Config
	raceNumber  int
	fakeDiscord *FakeDiscord
}

// SetupTest starts a fake discord and generates the plugin configuration.
// It is run for each test.
func (s *DiscordBaseSuite) SetupTest() {
	t := s.T()

	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)
	s.raceNumber = runtime.GOMAXPROCS(0)

	s.fakeDiscord = NewFakeDiscord(s.raceNumber)
	t.Cleanup(s.fakeDiscord.Close)

	var conf discord.Config
	conf.Teleport = s.TeleportConfig()
	conf.Discord.Token = "000000"
	conf.Discord.APIURL = s.fakeDiscord.URL() + "/"
	conf.PluginType = types.PluginTypeDiscord

	s.appConfig = &conf
}

// startApp starts the discord plugin, waits for it to become ready and returns,
func (s *DiscordBaseSuite) startApp() {
	t := s.T()
	t.Helper()

	app := discord.NewApp(s.appConfig)
	integration.RunAndWaitReady(t, app)
}

// DiscordSuiteOSS contains all tests that support running against a Teleport
// OSS Server.
type DiscordSuiteOSS struct {
	DiscordBaseSuite
}

// DiscordSuiteEnterprise contains all tests that require a Teleport Enterprise
// to run.
type DiscordSuiteEnterprise struct {
	DiscordBaseSuite
}

// SetupTest overrides DiscordBaseSuite.SetupTest to check the Teleport features
// before each test.
func (s *DiscordSuiteEnterprise) SetupTest() {
	t := s.T()
	s.RequireAdvancedWorkflow(t)
	s.DiscordBaseSuite.SetupTest()
}

// TestMessagePosting validates that a message is sent to each recipient
// specified in the plugin's configuration. It also checks that the message
// content is correct.
func (s *DiscordSuiteOSS) TestMessagePosting() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// max size of request was decreased here: https://github.com/gravitational/teleport/pull/13298
	s.SetReasonPadding(4000)

	// When we define two recipients for the editor role requests.
	s.appConfig.Recipients = common.RawRecipientsMap{
		"editor": []string{
			"1001", // recipient 1
			"1002", // recipient 2
		},
		"*": []string{"fallback"},
	}

	s.startApp()
	userName := integration.RequesterOSSUserName
	req := s.CreateAccessRequest(ctx, userName, nil)

	// We expect 2 messages to be sent by the plugin: one for each recipient.
	// We check if the stored plugin data makes sense.
	pluginData := s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})
	assert.Len(t, pluginData.SentMessages, 2)

	// Then we check that our fake Discord has received the messages.
	var messages []discord.DiscordMsg
	messageSet := make(MessageSet)
	for range 2 {
		msg, err := s.fakeDiscord.CheckNewMessage(ctx)
		require.NoError(t, err)
		messageSet.Add(accessrequest.MessageData{ChannelID: msg.Channel, MessageID: msg.DiscordID})
		messages = append(messages, msg)
	}

	assert.Len(t, messageSet, 2)
	assert.Contains(t, messageSet, pluginData.SentMessages[0])
	assert.Contains(t, messageSet, pluginData.SentMessages[1])

	// Finally, we validate the messages content
	sort.Sort(MessageSlice(messages))
	assert.Equal(t, s.appConfig.Recipients["editor"][0], messages[0].Channel)
	assert.Equal(t, s.appConfig.Recipients["editor"][1], messages[1].Channel)

	msgUser, err := parseMessageField(messages[0], "User")
	require.NoError(t, err)
	assert.Equal(t, integration.RequesterOSSUserName, msgUser)

	t.Logf("%q", messages[0].Text)
	matches := requestReasonRegexp.FindAllStringSubmatch(messages[0].Text, -1)
	require.Len(t, matches, 1)
	require.Len(t, matches[0], 3)
	assert.Equal(t, "because of "+strings.Repeat("A", 389), matches[0][1])
	assert.Equal(t, " (truncated)", matches[0][2])

	status, err := parseMessageField(messages[0], "Status")
	require.NoError(t, err)
	assert.Equal(t, "⏳ PENDING", status)
}

// TestApproval tests that when a request is approved, its corresponding message
// is updated to reflect the new request state.
func (s *DiscordSuiteOSS) TestApproval() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.appConfig.Recipients = common.RawRecipientsMap{
		"editor": []string{
			"1001", // reviewer 1
		},
		"*": []string{"fallback"},
	}

	s.startApp()

	// Test setup: we create an access request and wait for its Discord message
	userName := integration.RequesterOSSUserName
	req := s.CreateAccessRequest(ctx, userName, nil)
	msg, err := s.fakeDiscord.CheckNewMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, s.appConfig.Recipients["editor"][0], msg.Channel)

	// Test execution: we approve the request
	err = s.Ruler().ApproveAccessRequest(ctx, req.GetName(), "okay")
	require.NoError(t, err)

	// Validating the plugin updated the Discord message to reflect that it got approved
	msgUpdate, err := s.fakeDiscord.CheckMessageUpdateByAPI(ctx)
	require.NoError(t, err)
	assert.Equal(t, s.appConfig.Recipients["editor"][0], msgUpdate.Channel)
	assert.Equal(t, msg.DiscordID, msgUpdate.DiscordID)

	status, err := parseMessageField(msgUpdate, "Status")
	require.NoError(t, err)
	assert.Equal(t, "✅ APPROVED", status)
}

// TestDenial tests that when a request is denied, its corresponding message
// is updated to reflect the new request state.
func (s *DiscordSuiteOSS) TestDenial() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.appConfig.Recipients = common.RawRecipientsMap{
		"editor": []string{
			"1001", // reviewer 1
		},
		"*": []string{"fallback"},
	}

	s.startApp()

	// Test setup: we create an access request and wait for its Discord message
	userName := integration.RequesterOSSUserName
	req := s.CreateAccessRequest(ctx, userName, nil)
	msg, err := s.fakeDiscord.CheckNewMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, s.appConfig.Recipients["editor"][0], msg.Channel)

	// Test execution: we approve the request
	// max size of request was decreased here: https://github.com/gravitational/teleport/pull/13298
	err = s.Ruler().DenyAccessRequest(ctx, req.GetName(), "not okay "+strings.Repeat("A", 4000))
	require.NoError(t, err)

	// Validating the plugin updated the Discord message to reflect that it got denied
	msgUpdate, err := s.fakeDiscord.CheckMessageUpdateByAPI(ctx)
	require.NoError(t, err)
	assert.Equal(t, s.appConfig.Recipients["editor"][0], msgUpdate.Channel)
	assert.Equal(t, msg.DiscordID, msgUpdate.DiscordID)

	status, err := parseMessageField(msgUpdate, "Status")
	require.NoError(t, err)
	assert.Equal(t, "❌ DENIED", status) // Should fail
}

// TestReviewUpdates tests that the message is updated after the access request
// is reviewed. Each review should be reflected in the original message.
func (s *DiscordSuiteEnterprise) TestReviewUpdates() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.appConfig.Recipients = common.RawRecipientsMap{
		"editor": []string{
			"1001", // reviewer 1
		},
		"*": []string{"fallback"},
	}

	s.startApp()

	// Test setup: we create an access request and wait for its Discord message
	userName := integration.Requester1UserName
	req := s.CreateAccessRequest(ctx, userName, nil)

	s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})

	msg, err := s.fakeDiscord.CheckNewMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, s.appConfig.Recipients["editor"][0], msg.Channel)

	// Test execution: we submit a review and validate the message got updated
	err = s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	update, err := s.fakeDiscord.CheckMessageUpdateByAPI(ctx)
	require.NoError(t, err)
	assert.Equal(t, msg.Channel, update.Channel)
	assert.Equal(t, msg.DiscordID, update.DiscordID)
	assert.Equal(t, integration.Reviewer1UserName, update.Embeds[0].Author.Name, "embed must contain the review author")
	assert.Contains(t, update.Embeds[0].Title, "Approved request", "embed must contain a proposed state")
	assert.Contains(t, update.Embeds[0].Description, "Reason: ```\nokay```", "reply must contain a reason")

	// Test execution: we submit a second review and validate the message got updated again
	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	update, err = s.fakeDiscord.CheckMessageUpdateByAPI(ctx)
	require.NoError(t, err)
	assert.Equal(t, msg.Channel, update.Channel)
	assert.Equal(t, msg.DiscordID, update.DiscordID)
	assert.Equal(t, integration.Reviewer2UserName, update.Embeds[1].Author.Name, "embed must contain the review author")
	assert.Contains(t, update.Embeds[1].Title, "Denied request", "embed must contain a proposed state")
	assert.Contains(t, update.Embeds[1].Description, "Reason: ```\nnot okay```", "reply must contain a reason")
}

// TestApprovalByReview tests that the message is updated after the access request
// is reviewed and approved.
func (s *DiscordSuiteEnterprise) TestApprovalByReview() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.appConfig.Recipients = common.RawRecipientsMap{
		"editor": []string{
			"1001", // reviewer 1
		},
		"*": []string{"fallback"},
	}

	s.startApp()

	// Test setup: we create an access request and wait for its Discord message
	userName := integration.Requester1UserName
	req := s.CreateAccessRequest(ctx, userName, nil)

	s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})

	msg, err := s.fakeDiscord.CheckNewMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, s.appConfig.Recipients["editor"][0], msg.Channel)

	// Test execution: we submit a review and validate the message got updated
	err = s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	update, err := s.fakeDiscord.CheckMessageUpdateByAPI(ctx)
	require.NoError(t, err)
	assert.Equal(t, msg.Channel, update.Channel)
	assert.Equal(t, msg.DiscordID, update.DiscordID)
	assert.Equal(t, integration.Reviewer1UserName, update.Embeds[0].Author.Name, "embed must contain the review author")

	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "finally okay",
	})
	require.NoError(t, err)

	// Test execution: we submit a second review and validate the message got updated.
	// As the second review meets the approval threshold, the message status
	// should now be "approved".
	update, err = s.fakeDiscord.CheckMessageUpdateByAPI(ctx)
	require.NoError(t, err)
	assert.Equal(t, msg.Channel, update.Channel)
	assert.Equal(t, msg.DiscordID, update.DiscordID)
	assert.Equal(t, integration.Reviewer2UserName, update.Embeds[1].Author.Name, "embed must contain the review author")
	status, err := parseMessageField(update, "Status")
	require.NoError(t, err)
	assert.Equal(t, "✅ APPROVED", status)
}

// TestDenialByReview tests that the message is updated after the access request
// is reviewed and denied.
func (s *DiscordSuiteEnterprise) TestDenialByReview() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.appConfig.Recipients = common.RawRecipientsMap{
		"editor": []string{
			"1001", // reviewer 1
		},
		"*": []string{"fallback"},
	}

	s.startApp()

	// Test setup: we create an access request and wait for its Discord message
	userName := integration.Requester1UserName
	req := s.CreateAccessRequest(ctx, userName, nil)

	s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})

	msg, err := s.fakeDiscord.CheckNewMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, s.appConfig.Recipients["editor"][0], msg.Channel)

	// Test execution: we submit a review and validate the message got updated
	err = s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	update, err := s.fakeDiscord.CheckMessageUpdateByAPI(ctx)
	require.NoError(t, err)
	assert.Equal(t, msg.Channel, update.Channel)
	assert.Equal(t, msg.DiscordID, update.DiscordID)
	assert.Equal(t, integration.Reviewer1UserName, update.Embeds[0].Author.Name, "embed must contain the review author")

	// Test execution: we submit a second review and validate the message got updated.
	// As the second review meets the denial threshold, the message status
	// should now be "denied".
	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "finally not okay",
	})
	require.NoError(t, err)

	update, err = s.fakeDiscord.CheckMessageUpdateByAPI(ctx)
	require.NoError(t, err)
	assert.Equal(t, msg.Channel, update.Channel)
	assert.Equal(t, msg.DiscordID, update.DiscordID)
	assert.Equal(t, integration.Reviewer2UserName, update.Embeds[1].Author.Name, "embed must contain the review author")
	status, err := parseMessageField(update, "Status")
	require.NoError(t, err)
	assert.Equal(t, "❌ DENIED", status)
}

// TestExpiration tests that when a request expires, its corresponding message
// is updated to reflect the new request state.
func (s *DiscordSuiteOSS) TestExpiration() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.appConfig.Recipients = common.RawRecipientsMap{
		"editor": []string{
			"1001", // reviewer 1
		},
		"*": []string{"fallback"},
	}

	s.startApp()

	// Test setup: we create an access request and wait for its Discord message
	userName := integration.RequesterOSSUserName
	req := s.CreateAccessRequest(ctx, userName, nil)

	s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})

	msg, err := s.fakeDiscord.CheckNewMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, s.appConfig.Recipients["editor"][0], msg.Channel)

	// Test execution: we expire the request
	err = s.Ruler().DeleteAccessRequest(ctx, req.GetName()) // simulate expiration
	require.NoError(t, err)

	// Validating the plugin updated the Discord message to reflect that the request expired
	update, err := s.fakeDiscord.CheckMessageUpdateByAPI(ctx)
	require.NoError(t, err)
	assert.Equal(t, msg.Channel, update.Channel)
	assert.Equal(t, msg.DiscordID, update.DiscordID)

	status, err := parseMessageField(update, "Status")
	require.NoError(t, err)
	assert.Equal(t, "⌛ EXPIRED", status)
}

// TestRace validates that the plugin behaves properly and performs all the
// message updates when a lot of access requests are sent and reviewed in a very
// short time frame.
func (s *DiscordSuiteEnterprise) TestRace() {
	t := s.T()
	t.Skip("This test is flaky")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	err := logger.Setup(logger.Config{Severity: "info"}) // Turn off noisy debug logging
	require.NoError(t, err)

	s.appConfig.Recipients = common.RawRecipientsMap{
		"editor": []string{
			"1001", // reviewer 1
			"1002", // reviewer 2
		},
		"*": []string{"fallback"},
	}

	s.startApp()

	var (
		raceErr           error
		raceErrOnce       sync.Once
		threadMsgIDs      sync.Map
		threadMsgsCount   int32
		msgUpdateCounters sync.Map
	)
	setRaceErr := func(err error) error {
		raceErrOnce.Do(func() {
			raceErr = err
		})
		return err
	}

	// We create X access requests, this will send 2*X messages as "editor" has two recipients
	process := lib.NewProcess(ctx)
	for range s.raceNumber {
		process.SpawnCritical(func(ctx context.Context) error {
			req, err := types.NewAccessRequest(uuid.New().String(), integration.Requester1UserName, "editor")
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			if _, err := s.Requester1().CreateAccessRequestV2(ctx, req); err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			return nil
		})
	}

	// We start 2*X processes, each one will consume a message and approve it
	for range 2 * s.raceNumber {
		process.SpawnCritical(func(ctx context.Context) error {
			msg, err := s.fakeDiscord.CheckNewMessage(ctx)
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}

			threadMsgKey := accessrequest.MessageData{ChannelID: msg.Channel, MessageID: msg.DiscordID}
			if _, loaded := threadMsgIDs.LoadOrStore(threadMsgKey, struct{}{}); loaded {
				return setRaceErr(trace.Errorf("thread %v already stored", threadMsgKey))
			}
			atomic.AddInt32(&threadMsgsCount, 1)

			var user string
			switch msg.Channel {
			case "1001":
				user = integration.Reviewer1UserName
			case "1002":
				user = integration.Reviewer2UserName
			}

			reqID, err := parseMessageField(msg, "ID")
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}

			if err = s.ClientByName(user).SubmitAccessRequestReview(ctx, reqID, types.AccessReview{
				Author:        user,
				ProposedState: types.RequestState_APPROVED,
				Created:       time.Now(),
				Reason:        "okay",
			}); err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			return nil
		})
	}

	// All the access requests should have been approved. Each approval triggers a message update.
	// As each access requests has 2 messages, this gives us 4 updates per access requests.
	// We consume all updates and fill counters.
	for range 2 * 2 * s.raceNumber {
		process.SpawnCritical(func(ctx context.Context) error {
			msg, err := s.fakeDiscord.CheckMessageUpdateByAPI(ctx)
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}

			threadMsgKey := accessrequest.MessageData{ChannelID: msg.Channel, MessageID: msg.DiscordID}
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

	// We check each message was updated twice by using counters computed previously
	assert.Equal(t, int32(2*s.raceNumber), threadMsgsCount)
	threadMsgIDs.Range(func(key, value any) bool {
		next := true
		val, loaded := msgUpdateCounters.LoadAndDelete(key)
		next = next && assert.True(t, loaded)
		counterPtr := val.(*int32)
		next = next && assert.Equal(t, int32(2), *counterPtr)

		return next
	})
}

// TestMessagePostingWithAMR validates that a message is sent to each recipient
// specified in the monitoring rule and the plugin config is ignored. It also checks that the message
// content is correct.
func (s *DiscordSuiteOSS) TestMessagePostingWithAMR() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// max size of request was decreased here: https://github.com/gravitational/teleport/pull/13298
	s.SetReasonPadding(4000)

	// When we define two recipients for the editor role requests.
	s.appConfig.Recipients = common.RawRecipientsMap{
		"editor": []string{
			"1001", // recipient 1
		},
		"*": []string{"fallback"},
	}

	s.startApp()

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
					Name: "discord",
					Recipients: []string{
						"1001", // recipient 1
						"1002", // recipient 2
					},
				},
			},
		})
	assert.NoError(t, err)

	userName := integration.RequesterOSSUserName
	req := s.CreateAccessRequest(ctx, userName, nil)

	// We expect 2 messages to be sent by the plugin: one for each recipient.
	// We check if the stored plugin data makes sense.
	pluginData := s.checkPluginData(ctx, req.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})
	assert.Len(t, pluginData.SentMessages, 2)

	// Then we check that our fake Discord has received the messages.
	var messages []discord.DiscordMsg
	messageSet := make(MessageSet)
	for range 2 {
		msg, err := s.fakeDiscord.CheckNewMessage(ctx)
		require.NoError(t, err)
		messageSet.Add(accessrequest.MessageData{ChannelID: msg.Channel, MessageID: msg.DiscordID})
		messages = append(messages, msg)
	}

	assert.Len(t, messageSet, 2)
	assert.Contains(t, messageSet, pluginData.SentMessages[0])
	assert.Contains(t, messageSet, pluginData.SentMessages[1])

	// Finally, we validate the messages content
	sort.Sort(MessageSlice(messages))
	assert.Equal(t, "1001", messages[0].Channel)
	assert.Equal(t, "1002", messages[1].Channel)

	msgUser, err := parseMessageField(messages[0], "User")
	require.NoError(t, err)
	assert.Equal(t, integration.RequesterOSSUserName, msgUser)

	t.Logf("%q", messages[0].Text)
	matches := requestReasonRegexp.FindAllStringSubmatch(messages[0].Text, -1)
	require.Len(t, matches, 1)
	require.Len(t, matches[0], 3)
	assert.Equal(t, "because of "+strings.Repeat("A", 389), matches[0][1])
	assert.Equal(t, " (truncated)", matches[0][2])

	status, err := parseMessageField(messages[0], "Status")
	require.NoError(t, err)
	assert.Equal(t, "⏳ PENDING", status)
}
