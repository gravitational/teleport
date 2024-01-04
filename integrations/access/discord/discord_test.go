/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package discord

import (
	"context"
	"os/user"
	"regexp"
	"runtime"
	"sort"
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
	"github.com/gravitational/teleport/integrations/access/accessrequest"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
)

var msgFieldRegexp = regexp.MustCompile(`(?im)^\*([a-zA-Z ]+)\*: (.+)$`)
var requestReasonRegexp = regexp.MustCompile("(?im)^\\*Reason\\*:\\ ```\\n(.*?)```(.*?)$")

type DiscordSuite struct {
	integration.Suite
	appConfig *Config
	userNames struct {
		ruler     string
		requestor string
		reviewer1 string
		reviewer2 string
		plugin    string
	}
	raceNumber  int
	fakeDiscord *FakeDiscord

	clients          map[string]*integration.Client
	teleportFeatures *proto.Features
	teleportConfig   lib.TeleportConfig
}

func TestDiscordBot(t *testing.T) { suite.Run(t, &DiscordSuite{}) }

func (s *DiscordSuite) SetupSuite() {
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
		conditions.Request.Thresholds = []types.AccessReviewThreshold{types.AccessReviewThreshold{Approve: 2, Deny: 2}}
	}
	role, err := bootstrap.AddRole("foo", types.RoleSpecV6{Allow: conditions})
	require.NoError(t, err)

	user, err := bootstrap.AddUserWithRoles(me.Username+"@example.com", role.GetName())
	require.NoError(t, err)
	s.userNames.requestor = user.GetName()

	// Set up TWO users who can review access requests to role "editor".

	conditions = types.RoleConditions{}
	if teleportFeatures.AdvancedAccessWorkflows {
		conditions.ReviewRequests = &types.AccessReviewConditions{Roles: []string{"editor"}}
	}
	role, err = bootstrap.AddRole("foo-reviewer", types.RoleSpecV6{Allow: conditions})
	require.NoError(t, err)

	user, err = bootstrap.AddUserWithRoles(me.Username+"-reviewer1@example.com", role.GetName())
	require.NoError(t, err)
	s.userNames.reviewer1 = user.GetName()

	user, err = bootstrap.AddUserWithRoles(me.Username+"-reviewer2@example.com", role.GetName())
	require.NoError(t, err)
	s.userNames.reviewer2 = user.GetName()

	// Set up plugin user.
	role, err = bootstrap.AddRole("access-discord", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule("access_request", []string{"list", "read"}),
				types.NewRule("access_plugin_data", []string{"update"}),
			},
		},
	})
	require.NoError(t, err)

	user, err = bootstrap.AddUserWithRoles("access-discord", role.GetName())
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

func (s *DiscordSuite) SetupTest() {
	t := s.T()

	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)

	s.fakeDiscord = NewFakeDiscord(s.raceNumber)
	t.Cleanup(s.fakeDiscord.Close)

	var conf Config
	conf.Teleport = s.teleportConfig
	conf.Discord.Token = "000000"
	conf.Discord.APIURL = s.fakeDiscord.URL() + "/"

	s.appConfig = &conf
	s.SetContextTimeout(5 * time.Second)
}

func (s *DiscordSuite) startApp() {
	t := s.T()
	t.Helper()

	app := NewApp(s.appConfig)
	s.StartApp(app)
}

func (s *DiscordSuite) ruler() *integration.Client {
	return s.clients[s.userNames.ruler]
}

func (s *DiscordSuite) requestor() *integration.Client {
	return s.clients[s.userNames.requestor]
}

func (s *DiscordSuite) reviewer1() *integration.Client {
	return s.clients[s.userNames.reviewer1]
}

func (s *DiscordSuite) reviewer2() *integration.Client {
	return s.clients[s.userNames.reviewer2]
}

func (s *DiscordSuite) newAccessRequest() types.AccessRequest {
	t := s.T()
	t.Helper()

	req, err := types.NewAccessRequest(uuid.New().String(), s.userNames.requestor, "editor")
	require.NoError(t, err)
	// max size of request was decreased here: https://github.com/gravitational/teleport/pull/13298
	req.SetRequestReason("because of " + strings.Repeat("A", 4000))
	return req
}

func (s *DiscordSuite) createAccessRequest() types.AccessRequest {
	t := s.T()
	t.Helper()

	req := s.newAccessRequest()
	out, err := s.requestor().CreateAccessRequestV2(s.Context(), req)
	require.NoError(t, err)
	return out
}

func (s *DiscordSuite) checkPluginData(reqID string, cond func(accessrequest.PluginData) bool) accessrequest.PluginData {
	t := s.T()
	t.Helper()

	for {
		rawData, err := s.ruler().PollAccessRequestPluginData(s.Context(), "discord", reqID)
		require.NoError(t, err)
		data, err := accessrequest.DecodePluginData(rawData)
		require.NoError(t, err)
		if cond(data) {
			return data
		}
	}
}

func (s *DiscordSuite) TestMessagePosting() {
	t := s.T()

	s.appConfig.Recipients = common.RawRecipientsMap{
		"editor": []string{
			"1001", // reviewer 1
			"1002", // reviewer 2
		},
		"*": []string{"fallback"},
	}

	s.startApp()
	request := s.createAccessRequest()

	pluginData := s.checkPluginData(request.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})
	assert.Len(t, pluginData.SentMessages, 2)

	var messages []DiscordMsg
	messageSet := make(MessageSet)
	for i := 0; i < 2; i++ {
		msg, err := s.fakeDiscord.CheckNewMessage(s.Context())
		require.NoError(t, err)
		messageSet.Add(accessrequest.MessageData{ChannelID: msg.Channel, MessageID: msg.DiscordID})
		messages = append(messages, msg)
	}

	assert.Len(t, messageSet, 2)
	assert.Contains(t, messageSet, pluginData.SentMessages[0])
	assert.Contains(t, messageSet, pluginData.SentMessages[1])

	sort.Sort(MessageSlice(messages))

	assert.Equal(t, s.appConfig.Recipients["editor"][0], messages[0].Channel)
	assert.Equal(t, s.appConfig.Recipients["editor"][1], messages[1].Channel)

	msgUser, err := parseMessageField(messages[0], "User")
	require.NoError(t, err)
	assert.Equal(t, s.userNames.requestor, msgUser)

	t.Logf("%q", messages[0].Text)
	matches := requestReasonRegexp.FindAllStringSubmatch(messages[0].Text, -1)
	require.Len(t, matches, 1)
	require.Len(t, matches[0], 3)
	assert.Equal(t, "because of "+strings.Repeat("A", 489), matches[0][1])
	assert.Equal(t, " (truncated)", matches[0][2])

	status, err := parseMessageField(messages[0], "Status")
	require.NoError(t, err)
	assert.Equal(t, "⏳ PENDING", status)
}

func (s *DiscordSuite) TestApproval() {
	t := s.T()

	s.appConfig.Recipients = common.RawRecipientsMap{
		"editor": []string{
			"1001", // reviewer 1
		},
		"*": []string{"fallback"},
	}

	s.startApp()

	req := s.createAccessRequest()
	msg, err := s.fakeDiscord.CheckNewMessage(s.Context())
	require.NoError(t, err)
	assert.Equal(t, s.appConfig.Recipients["editor"][0], msg.Channel)

	err = s.ruler().ApproveAccessRequest(s.Context(), req.GetName(), "okay")
	require.NoError(t, err)

	msgUpdate, err := s.fakeDiscord.CheckMessageUpdateByAPI(s.Context())
	require.NoError(t, err)
	assert.Equal(t, s.appConfig.Recipients["editor"][0], msgUpdate.Channel)
	assert.Equal(t, msg.DiscordID, msgUpdate.DiscordID)

	status, err := parseMessageField(msgUpdate, "Status")
	require.NoError(t, err)
	assert.Equal(t, "✅ APPROVED", status) // Should fail
}

func (s *DiscordSuite) TestDenial() {
	t := s.T()

	s.appConfig.Recipients = common.RawRecipientsMap{
		"editor": []string{
			"1001", // reviewer 1
		},
		"*": []string{"fallback"},
	}

	s.startApp()

	req := s.createAccessRequest()
	msg, err := s.fakeDiscord.CheckNewMessage(s.Context())
	require.NoError(t, err)
	assert.Equal(t, s.appConfig.Recipients["editor"][0], msg.Channel)

	// max size of request was decreased here: https://github.com/gravitational/teleport/pull/13298
	err = s.ruler().DenyAccessRequest(s.Context(), req.GetName(), "not okay "+strings.Repeat("A", 4000))
	require.NoError(t, err)

	msgUpdate, err := s.fakeDiscord.CheckMessageUpdateByAPI(s.Context())
	require.NoError(t, err)
	assert.Equal(t, s.appConfig.Recipients["editor"][0], msgUpdate.Channel)
	assert.Equal(t, msg.DiscordID, msgUpdate.DiscordID)

	status, err := parseMessageField(msgUpdate, "Status")
	require.NoError(t, err)
	assert.Equal(t, "❌ DENIED", status) // Should fail
}

func (s *DiscordSuite) TestReviewUpdates() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	s.appConfig.Recipients = common.RawRecipientsMap{
		"editor": []string{
			"1001", // reviewer 1
		},
		"*": []string{"fallback"},
	}

	s.startApp()

	request := s.createAccessRequest()

	s.checkPluginData(request.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})

	msg, err := s.fakeDiscord.CheckNewMessage(s.Context())
	require.NoError(t, err)
	assert.Equal(t, s.appConfig.Recipients["editor"][0], msg.Channel)

	err = s.reviewer1().SubmitAccessRequestReview(s.Context(), request.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer1,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	update, err := s.fakeDiscord.CheckMessageUpdateByAPI(s.Context())
	require.NoError(t, err)
	assert.Equal(t, msg.Channel, update.Channel)
	assert.Equal(t, msg.DiscordID, update.DiscordID)
	assert.Equal(t, update.Embeds[0].Author.Name, s.userNames.reviewer1, "embed must contain the review author")
	assert.Contains(t, update.Embeds[0].Title, "Approved request", "embed must contain a proposed state")
	assert.Contains(t, update.Embeds[0].Description, "Reason: ```\nokay```", "reply must contain a reason")

	err = s.reviewer2().SubmitAccessRequestReview(s.Context(), request.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer2,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	update, err = s.fakeDiscord.CheckMessageUpdateByAPI(s.Context())
	require.NoError(t, err)
	assert.Equal(t, msg.Channel, update.Channel)
	assert.Equal(t, msg.DiscordID, update.DiscordID)
	assert.Equal(t, update.Embeds[1].Author.Name, s.userNames.reviewer2, "embed must contain the review author")
	assert.Contains(t, update.Embeds[1].Title, "Denied request", "embed must contain a proposed state")
	assert.Contains(t, update.Embeds[1].Description, "Reason: ```\nnot okay```", "reply must contain a reason")
}

func (s *DiscordSuite) TestApprovalByReview() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	s.appConfig.Recipients = common.RawRecipientsMap{
		"editor": []string{
			"1001", // reviewer 1
		},
		"*": []string{"fallback"},
	}

	s.startApp()

	request := s.createAccessRequest()

	s.checkPluginData(request.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})

	msg, err := s.fakeDiscord.CheckNewMessage(s.Context())
	require.NoError(t, err)
	assert.Equal(t, s.appConfig.Recipients["editor"][0], msg.Channel)

	err = s.reviewer1().SubmitAccessRequestReview(s.Context(), request.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer1,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	update, err := s.fakeDiscord.CheckMessageUpdateByAPI(s.Context())
	require.NoError(t, err)
	assert.Equal(t, msg.Channel, update.Channel)
	assert.Equal(t, msg.DiscordID, update.DiscordID)
	assert.Equal(t, update.Embeds[0].Author.Name, s.userNames.reviewer1, "embed must contain the review author")

	err = s.reviewer2().SubmitAccessRequestReview(s.Context(), request.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer2,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "finally okay",
	})
	require.NoError(t, err)

	update, err = s.fakeDiscord.CheckMessageUpdateByAPI(s.Context())
	require.NoError(t, err)
	assert.Equal(t, msg.Channel, update.Channel)
	assert.Equal(t, msg.DiscordID, update.DiscordID)
	assert.Equal(t, update.Embeds[1].Author.Name, s.userNames.reviewer2, "embed must contain the review author")
	status, err := parseMessageField(update, "Status")
	require.NoError(t, err)
	assert.Equal(t, "✅ APPROVED", status)
}

func (s *DiscordSuite) TestDenialByReview() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	s.appConfig.Recipients = common.RawRecipientsMap{
		"editor": []string{
			"1001", // reviewer 1
		},
		"*": []string{"fallback"},
	}

	s.startApp()

	request := s.createAccessRequest()

	s.checkPluginData(request.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})

	msg, err := s.fakeDiscord.CheckNewMessage(s.Context())
	require.NoError(t, err)
	assert.Equal(t, s.appConfig.Recipients["editor"][0], msg.Channel)

	err = s.reviewer1().SubmitAccessRequestReview(s.Context(), request.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer1,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	update, err := s.fakeDiscord.CheckMessageUpdateByAPI(s.Context())
	require.NoError(t, err)
	assert.Equal(t, msg.Channel, update.Channel)
	assert.Equal(t, msg.DiscordID, update.DiscordID)
	assert.Equal(t, update.Embeds[0].Author.Name, s.userNames.reviewer1, "embed must contain the review author")

	err = s.reviewer2().SubmitAccessRequestReview(s.Context(), request.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer2,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "finally not okay",
	})
	require.NoError(t, err)

	update, err = s.fakeDiscord.CheckMessageUpdateByAPI(s.Context())
	require.NoError(t, err)
	assert.Equal(t, msg.Channel, update.Channel)
	assert.Equal(t, msg.DiscordID, update.DiscordID)
	assert.Equal(t, update.Embeds[1].Author.Name, s.userNames.reviewer2, "embed must contain the review author")
	status, err := parseMessageField(update, "Status")
	require.NoError(t, err)
	assert.Equal(t, "❌ DENIED", status)
}

func (s *DiscordSuite) TestExpiration() {
	t := s.T()

	s.appConfig.Recipients = common.RawRecipientsMap{
		"editor": []string{
			"1001", // reviewer 1
		},
		"*": []string{"fallback"},
	}

	s.startApp()

	request := s.createAccessRequest()

	s.checkPluginData(request.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})

	msg, err := s.fakeDiscord.CheckNewMessage(s.Context())
	require.NoError(t, err)
	assert.Equal(t, s.appConfig.Recipients["editor"][0], msg.Channel)

	s.checkPluginData(request.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})

	err = s.ruler().DeleteAccessRequest(s.Context(), request.GetName()) // simulate expiration
	require.NoError(t, err)

	update, err := s.fakeDiscord.CheckMessageUpdateByAPI(s.Context())
	require.NoError(t, err)
	assert.Equal(t, msg.Channel, update.Channel)
	assert.Equal(t, msg.DiscordID, update.DiscordID)

	status, err := parseMessageField(update, "Status")
	require.NoError(t, err)
	assert.Equal(t, "⌛ EXPIRED", status)
}

func (s *DiscordSuite) TestRace() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	err := logger.Setup(logger.Config{Severity: "info"}) // Turn off noisy debug logging
	require.NoError(t, err)

	s.appConfig.Recipients = common.RawRecipientsMap{
		"editor": []string{
			"1001", // reviewer 1
			"1002", // reviewer 2
		},
		"*": []string{"fallback"},
	}

	s.SetContextTimeout(20 * time.Second)
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
	process := lib.NewProcess(s.Context())
	for i := 0; i < s.raceNumber; i++ {
		process.SpawnCritical(func(ctx context.Context) error {
			req, err := types.NewAccessRequest(uuid.New().String(), s.userNames.requestor, "editor")
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			if _, err := s.requestor().CreateAccessRequestV2(ctx, req); err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			return nil
		})
	}

	// We start 2*X processes, each one will consume a message and approve it
	for i := 0; i < 2*s.raceNumber; i++ {
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
				user = s.userNames.reviewer1
			case "1002":
				user = s.userNames.reviewer2
			}

			reqID, err := parseMessageField(msg, "ID")
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}

			if err = s.clients[user].SubmitAccessRequestReview(ctx, reqID, types.AccessReview{
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
	for i := 0; i < 2*2*s.raceNumber; i++ {
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
	threadMsgIDs.Range(func(key, value interface{}) bool {
		next := true
		val, loaded := msgUpdateCounters.LoadAndDelete(key)
		next = next && assert.True(t, loaded)
		counterPtr := val.(*int32)
		next = next && assert.Equal(t, int32(2), *counterPtr)

		return next
	})
}

func parseMessageField(msg DiscordMsg, field string) (string, error) {
	if msg.Text == "" {
		return "", trace.Errorf("message does not contain text")
	}

	matches := msgFieldRegexp.FindAllStringSubmatch(msg.Text, -1)
	if matches == nil {
		return "", trace.Errorf("cannot parse fields from text %s", msg.Text)
	}
	var fields []string
	for _, match := range matches {
		if match[1] == field {
			return match[2], nil
		}
		fields = append(fields, match[1])
	}
	return "", trace.Errorf("cannot find field %s in %v", field, fields)
}
