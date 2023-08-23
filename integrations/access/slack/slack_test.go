// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package slack

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
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/auth"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
)

var msgFieldRegexp = regexp.MustCompile(`(?im)^\*([a-zA-Z ]+)\*: (.+)$`)
var requestReasonRegexp = regexp.MustCompile("(?im)^\\*Reason\\*:\\ ```\\n(.*?)```(.*?)$")

type SlackSuite struct {
	integration.Suite
	appConfig *Config
	userNames struct {
		ruler     string
		requestor string
		reviewer1 string
		reviewer2 string
		plugin    string
	}
	raceNumber     int
	fakeSlack      *FakeSlack
	fakeStatusSink *fakeStatusSink

	clients          map[string]*integration.Client
	teleportFeatures *proto.Features
	teleportConfig   lib.TeleportConfig
}

func TestSlackbot(t *testing.T) { suite.Run(t, &SlackSuite{}) }

func (s *SlackSuite) SetupSuite() {
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

	role, err = bootstrap.AddRole("access-slack", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule("access_request", []string{"list", "read"}),
				types.NewRule("access_plugin_data", []string{"update"}),
			},
		},
	})
	require.NoError(t, err)

	user, err = bootstrap.AddUserWithRoles("access-slack", role.GetName())
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

func (s *SlackSuite) SetupTest() {
	t := s.T()

	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)

	s.fakeSlack = NewFakeSlack(User{Name: "slackbot"}, s.raceNumber)
	t.Cleanup(s.fakeSlack.Close)

	s.fakeSlack.StoreUser(User{Name: "Vladimir", Profile: UserProfile{Email: s.userNames.requestor}})

	s.fakeStatusSink = &fakeStatusSink{}

	var conf Config
	conf.Teleport = s.teleportConfig
	conf.Slack.Token = "000000"
	conf.Slack.APIURL = s.fakeSlack.URL() + "/"
	conf.AccessTokenProvider = auth.NewStaticAccessTokenProvider(conf.Slack.Token)
	conf.StatusSink = s.fakeStatusSink

	s.appConfig = &conf
	s.SetContextTimeout(5 * time.Second)
}

func (s *SlackSuite) startApp() {
	t := s.T()
	t.Helper()

	app := NewSlackApp(s.appConfig)
	s.StartApp(app)
}

func (s *SlackSuite) ruler() *integration.Client {
	return s.clients[s.userNames.ruler]
}

func (s *SlackSuite) requestor() *integration.Client {
	return s.clients[s.userNames.requestor]
}

func (s *SlackSuite) reviewer1() *integration.Client {
	return s.clients[s.userNames.reviewer1]
}

func (s *SlackSuite) reviewer2() *integration.Client {
	return s.clients[s.userNames.reviewer2]
}

func (s *SlackSuite) newAccessRequest(reviewers []User) types.AccessRequest {
	t := s.T()
	t.Helper()

	req, err := types.NewAccessRequest(uuid.New().String(), s.userNames.requestor, "editor")
	require.NoError(t, err)
	// max size of request was decreased here: https://github.com/gravitational/teleport/pull/13298
	req.SetRequestReason("because of " + strings.Repeat("A", 4000))
	var suggestedReviewers []string
	for _, user := range reviewers {
		suggestedReviewers = append(suggestedReviewers, user.Profile.Email)
	}
	req.SetSuggestedReviewers(suggestedReviewers)
	return req
}

func (s *SlackSuite) createAccessRequest(reviewers []User) types.AccessRequest {
	t := s.T()
	t.Helper()

	req := s.newAccessRequest(reviewers)
	err := s.requestor().CreateAccessRequest(s.Context(), req)
	require.NoError(t, err)
	return req
}

func (s *SlackSuite) checkPluginData(reqID string, cond func(common.GenericPluginData) bool) common.GenericPluginData {
	t := s.T()
	t.Helper()

	for {
		rawData, err := s.ruler().PollAccessRequestPluginData(s.Context(), "slack", reqID)
		require.NoError(t, err)
		data, err := common.DecodePluginData(rawData)
		require.NoError(t, err)
		if cond(data) {
			return data
		}
	}
}

func (s *SlackSuite) TestMessagePosting() {
	t := s.T()

	reviewer1 := s.fakeSlack.StoreUser(User{Profile: UserProfile{Email: s.userNames.reviewer1}})
	reviewer2 := s.fakeSlack.StoreUser(User{Profile: UserProfile{Email: s.userNames.reviewer2}})

	s.startApp()
	request := s.createAccessRequest([]User{reviewer2, reviewer1})

	pluginData := s.checkPluginData(request.GetName(), func(data common.GenericPluginData) bool {
		return len(data.SentMessages) > 0
	})
	assert.Len(t, pluginData.SentMessages, 2)

	var messages []Message
	messageSet := make(SlackDataMessageSet)
	for i := 0; i < 2; i++ {
		msg, err := s.fakeSlack.CheckNewMessage(s.Context())
		require.NoError(t, err)
		messageSet.Add(common.MessageData{ChannelID: msg.Channel, MessageID: msg.Timestamp})
		messages = append(messages, msg)
	}

	assert.Len(t, messageSet, 2)
	assert.Contains(t, messageSet, pluginData.SentMessages[0])
	assert.Contains(t, messageSet, pluginData.SentMessages[1])

	sort.Sort(SlackMessageSlice(messages))

	assert.Equal(t, reviewer1.ID, messages[0].Channel)
	assert.Equal(t, reviewer2.ID, messages[1].Channel)

	msgUser, err := parseMessageField(messages[0], "User")
	require.NoError(t, err)
	assert.Equal(t, s.userNames.requestor, msgUser)

	block, ok := messages[0].BlockItems[1].Block.(SectionBlock)
	require.True(t, ok)
	t.Logf("%q", block.Text.GetText())
	matches := requestReasonRegexp.FindAllStringSubmatch(block.Text.GetText(), -1)
	require.Equal(t, 1, len(matches))
	require.Equal(t, 3, len(matches[0]))
	assert.Equal(t, "because of "+strings.Repeat("A", 489), matches[0][1])
	assert.Equal(t, " (truncated)", matches[0][2])

	statusLine, err := getStatusLine(messages[0])
	require.NoError(t, err)
	assert.Equal(t, "*Status*: ⏳ PENDING", statusLine)

	assert.Equal(t, types.PluginStatusCode_RUNNING, s.fakeStatusSink.Get().GetCode())
}

func (s *SlackSuite) TestRecipientsConfig() {
	t := s.T()

	reviewer1 := s.fakeSlack.StoreUser(User{Profile: UserProfile{Email: s.userNames.reviewer1}})
	reviewer2 := s.fakeSlack.StoreUser(User{Profile: UserProfile{Email: s.userNames.reviewer2}})
	s.appConfig.Recipients = common.RawRecipientsMap{
		types.Wildcard: []string{reviewer2.Profile.Email, reviewer1.ID},
	}

	s.startApp()

	request := s.createAccessRequest(nil)
	pluginData := s.checkPluginData(request.GetName(), func(data common.GenericPluginData) bool {
		return len(data.SentMessages) > 0
	})
	assert.Len(t, pluginData.SentMessages, 2)

	var (
		msg      Message
		messages []Message
	)

	messageSet := make(SlackDataMessageSet)

	msg, err := s.fakeSlack.CheckNewMessage(s.Context())
	require.NoError(t, err)
	messageSet.Add(common.MessageData{ChannelID: msg.Channel, MessageID: msg.Timestamp})
	messages = append(messages, msg)

	msg, err = s.fakeSlack.CheckNewMessage(s.Context())
	require.NoError(t, err)
	messageSet.Add(common.MessageData{ChannelID: msg.Channel, MessageID: msg.Timestamp})
	messages = append(messages, msg)

	assert.Len(t, messageSet, 2)
	assert.Contains(t, messageSet, pluginData.SentMessages[0])
	assert.Contains(t, messageSet, pluginData.SentMessages[1])

	sort.Sort(SlackMessageSlice(messages))

	assert.Equal(t, reviewer1.ID, messages[0].Channel)
	assert.Equal(t, reviewer2.ID, messages[1].Channel)
}

func (s *SlackSuite) TestApproval() {
	t := s.T()

	reviewer := s.fakeSlack.StoreUser(User{Profile: UserProfile{Email: s.userNames.reviewer1}})

	s.startApp()

	req := s.createAccessRequest([]User{reviewer})
	msg, err := s.fakeSlack.CheckNewMessage(s.Context())
	require.NoError(t, err)
	assert.Equal(t, reviewer.ID, msg.Channel)

	err = s.ruler().ApproveAccessRequest(s.Context(), req.GetName(), "okay")
	require.NoError(t, err)

	msgUpdate, err := s.fakeSlack.CheckMessageUpdateByAPI(s.Context())
	require.NoError(t, err)
	assert.Equal(t, reviewer.ID, msgUpdate.Channel)
	assert.Equal(t, msg.Timestamp, msgUpdate.Timestamp)

	statusLine, err := getStatusLine(msgUpdate)
	require.NoError(t, err)
	assert.Equal(t, "*Status*: ✅ APPROVED\n*Resolution reason*: ```\nokay```", statusLine)
}

func (s *SlackSuite) TestDenial() {
	t := s.T()

	reviewer := s.fakeSlack.StoreUser(User{Profile: UserProfile{Email: s.userNames.reviewer1}})

	s.startApp()

	req := s.createAccessRequest([]User{reviewer})
	msg, err := s.fakeSlack.CheckNewMessage(s.Context())
	require.NoError(t, err)
	assert.Equal(t, reviewer.ID, msg.Channel)

	// max size of request was decreased here: https://github.com/gravitational/teleport/pull/13298
	err = s.ruler().DenyAccessRequest(s.Context(), req.GetName(), "not okay "+strings.Repeat("A", 4000))
	require.NoError(t, err)

	msgUpdate, err := s.fakeSlack.CheckMessageUpdateByAPI(s.Context())
	require.NoError(t, err)
	assert.Equal(t, reviewer.ID, msgUpdate.Channel)
	assert.Equal(t, msg.Timestamp, msgUpdate.Timestamp)

	statusLine, err := getStatusLine(msgUpdate)
	require.NoError(t, err)
	assert.Equal(t, "*Status*: ❌ DENIED\n*Resolution reason*: ```\nnot okay "+strings.Repeat("A", 491)+"``` (truncated)", statusLine)
}

func (s *SlackSuite) TestReviewReplies() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	reviewer := s.fakeSlack.StoreUser(User{Profile: UserProfile{Email: s.userNames.reviewer1}})

	s.startApp()

	req := s.createAccessRequest([]User{reviewer})
	s.checkPluginData(req.GetName(), func(data common.GenericPluginData) bool {
		return len(data.SentMessages) > 0
	})

	msg, err := s.fakeSlack.CheckNewMessage(s.Context())
	require.NoError(t, err)
	assert.Equal(t, reviewer.ID, msg.Channel)

	err = s.reviewer1().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer1,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	reply, err := s.fakeSlack.CheckNewMessage(s.Context())
	require.NoError(t, err)
	assert.Equal(t, msg.Channel, reply.Channel)
	assert.Equal(t, msg.Timestamp, reply.ThreadTs)
	assert.Contains(t, reply.Text, s.userNames.reviewer1+" reviewed the request", "reply must contain a review author")
	assert.Contains(t, reply.Text, "Resolution: ✅ APPROVED", "reply must contain a proposed state")
	assert.Contains(t, reply.Text, "Reason: ```\nokay```", "reply must contain a reason")

	err = s.reviewer2().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer2,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	reply, err = s.fakeSlack.CheckNewMessage(s.Context())
	require.NoError(t, err)
	assert.Equal(t, msg.Channel, reply.Channel)
	assert.Equal(t, msg.Timestamp, reply.ThreadTs)
	assert.Contains(t, reply.Text, s.userNames.reviewer2+" reviewed the request", "reply must contain a review author")
	assert.Contains(t, reply.Text, "Resolution: ❌ DENIED", "reply must contain a proposed state")
	assert.Contains(t, reply.Text, "Reason: ```\nnot okay```", "reply must contain a reason")
}

func (s *SlackSuite) TestApprovalByReview() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	reviewer := s.fakeSlack.StoreUser(User{Profile: UserProfile{Email: s.userNames.reviewer1}})

	s.startApp()

	req := s.createAccessRequest([]User{reviewer})
	msg, err := s.fakeSlack.CheckNewMessage(s.Context())
	require.NoError(t, err)
	assert.Equal(t, reviewer.ID, msg.Channel)

	err = s.reviewer1().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer1,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	reply, err := s.fakeSlack.CheckNewMessage(s.Context())
	require.NoError(t, err)
	assert.Equal(t, msg.Channel, reply.Channel)
	assert.Equal(t, msg.Timestamp, reply.ThreadTs)
	assert.Contains(t, reply.Text, s.userNames.reviewer1+" reviewed the request", "reply must contain a review author")

	err = s.reviewer2().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer2,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "finally okay",
	})
	require.NoError(t, err)

	reply, err = s.fakeSlack.CheckNewMessage(s.Context())
	require.NoError(t, err)
	assert.Equal(t, msg.Channel, reply.Channel)
	assert.Equal(t, msg.Timestamp, reply.ThreadTs)
	assert.Contains(t, reply.Text, s.userNames.reviewer2+" reviewed the request", "reply must contain a review author")
	// When posting a review, the slack bot also updates the message to add the amount of reviewrs
	// This update is soon superseded by the "access allowed" update
	_, _ = s.fakeSlack.CheckMessageUpdateByAPI(s.Context())

	msgUpdate, err := s.fakeSlack.CheckMessageUpdateByAPI(s.Context())
	require.NoError(t, err)
	assert.Equal(t, reviewer.ID, msgUpdate.Channel)
	assert.Equal(t, msg.Timestamp, msgUpdate.Timestamp)

	statusLine, err := getStatusLine(msgUpdate)
	require.NoError(t, err)
	assert.Equal(t, "*Status*: ✅ APPROVED\n*Resolution reason*: ```\nfinally okay```", statusLine)
}

func (s *SlackSuite) TestDenialByReview() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	reviewer := s.fakeSlack.StoreUser(User{Profile: UserProfile{Email: s.userNames.reviewer1}})

	s.startApp()

	req := s.createAccessRequest([]User{reviewer})
	msg, err := s.fakeSlack.CheckNewMessage(s.Context())
	require.NoError(t, err)
	assert.Equal(t, reviewer.ID, msg.Channel)

	err = s.reviewer1().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer1,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	reply, err := s.fakeSlack.CheckNewMessage(s.Context())
	require.NoError(t, err)
	assert.Equal(t, msg.Channel, reply.Channel)
	assert.Equal(t, msg.Timestamp, reply.ThreadTs)
	assert.Contains(t, reply.Text, s.userNames.reviewer1+" reviewed the request", "reply must contain a review author")

	err = s.reviewer2().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer2,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "finally not okay",
	})
	require.NoError(t, err)

	reply, err = s.fakeSlack.CheckNewMessage(s.Context())
	require.NoError(t, err)
	assert.Equal(t, msg.Channel, reply.Channel)
	assert.Equal(t, msg.Timestamp, reply.ThreadTs)
	assert.Contains(t, reply.Text, s.userNames.reviewer2+" reviewed the request", "reply must contain a review author")
	// When posting a review, the slack bot also updates the message to add the amount of reviewrs
	// This update is soon superseded by the "access allowed" update
	_, _ = s.fakeSlack.CheckMessageUpdateByAPI(s.Context())

	msgUpdate, err := s.fakeSlack.CheckMessageUpdateByAPI(s.Context())
	require.NoError(t, err)
	assert.Equal(t, reviewer.ID, msgUpdate.Channel)
	assert.Equal(t, msg.Timestamp, msgUpdate.Timestamp)

	statusLine, err := getStatusLine(msgUpdate)
	require.NoError(t, err)
	assert.Equal(t, "*Status*: ❌ DENIED\n*Resolution reason*: ```\nfinally not okay```", statusLine)
}

func (s *SlackSuite) TestExpiration() {
	t := s.T()

	reviewer := s.fakeSlack.StoreUser(User{Profile: UserProfile{Email: s.userNames.reviewer1}})

	s.startApp()

	request := s.createAccessRequest([]User{reviewer})
	msg, err := s.fakeSlack.CheckNewMessage(s.Context())
	require.NoError(t, err)
	assert.Equal(t, reviewer.ID, msg.Channel)

	s.checkPluginData(request.GetName(), func(data common.GenericPluginData) bool {
		return len(data.SentMessages) > 0
	})

	err = s.ruler().DeleteAccessRequest(s.Context(), request.GetName()) // simulate expiration
	require.NoError(t, err)

	msgUpdate, err := s.fakeSlack.CheckMessageUpdateByAPI(s.Context())
	require.NoError(t, err)
	assert.Equal(t, reviewer.ID, msgUpdate.Channel)
	assert.Equal(t, msg.Timestamp, msgUpdate.Timestamp)

	statusLine, err := getStatusLine(msgUpdate)
	require.NoError(t, err)
	assert.Equal(t, "*Status*: ⌛ EXPIRED", statusLine)
}

func (s *SlackSuite) TestRace() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	err := logger.Setup(logger.Config{Severity: "info"}) // Turn off noisy debug logging
	require.NoError(t, err)

	reviewer1 := s.fakeSlack.StoreUser(User{Profile: UserProfile{Email: s.userNames.reviewer1}})
	reviewer2 := s.fakeSlack.StoreUser(User{Profile: UserProfile{Email: s.userNames.reviewer2}})

	s.SetContextTimeout(20 * time.Second)
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

	process := lib.NewProcess(s.Context())
	for i := 0; i < s.raceNumber; i++ {
		process.SpawnCritical(func(ctx context.Context) error {
			req, err := types.NewAccessRequest(uuid.New().String(), s.userNames.requestor, "editor")
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			req.SetSuggestedReviewers([]string{reviewer1.Profile.Email, reviewer2.Profile.Email})
			if err := s.requestor().CreateAccessRequest(ctx, req); err != nil {
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
			msg, err := s.fakeSlack.CheckNewMessage(ctx)
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}

			if msg.ThreadTs == "" {
				// Handle "root" notifications.

				threadMsgKey := common.MessageData{ChannelID: msg.Channel, MessageID: msg.Timestamp}
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

				if err = s.clients[user.Profile.Email].SubmitAccessRequestReview(ctx, reqID, types.AccessReview{
					Author:        user.Profile.Email,
					ProposedState: types.RequestState_APPROVED,
					Created:       time.Now(),
					Reason:        "okay",
				}); err != nil {
					return setRaceErr(trace.Wrap(err))
				}
			} else {
				// Handle review comments.

				threadMsgKey := common.MessageData{ChannelID: msg.Channel, MessageID: msg.ThreadTs}
				var newCounter int32
				val, _ := reviewReplyCounters.LoadOrStore(threadMsgKey, &newCounter)
				counterPtr := val.(*int32)
				atomic.AddInt32(counterPtr, 1)
			}

			return nil
		})
	}

	// Multiplier TWO means that we handle the 2 updates for each of the two messages posted to reviewers.
	for i := 0; i < 2*2*s.raceNumber; i++ {
		process.SpawnCritical(func(ctx context.Context) error {
			msg, err := s.fakeSlack.CheckMessageUpdateByAPI(ctx)
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}

			threadMsgKey := common.MessageData{ChannelID: msg.Channel, MessageID: msg.Timestamp}
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

	assert.Equal(t, int32(2*s.raceNumber), threadMsgsCount)
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

func parseMessageField(msg Message, field string) (string, error) {
	block := msg.BlockItems[1].Block
	sectionBlock, ok := block.(SectionBlock)
	if !ok {
		return "", trace.Errorf("invalid block type %T", block)
	}

	if sectionBlock.Text.TextObject == nil {
		return "", trace.Errorf("section block does not contain text")
	}

	text := sectionBlock.Text.GetText()
	matches := msgFieldRegexp.FindAllStringSubmatch(text, -1)
	if matches == nil {
		return "", trace.Errorf("cannot parse fields from text %s", text)
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

func getStatusLine(msg Message) (string, error) {
	block := msg.BlockItems[2].Block
	contextBlock, ok := block.(ContextBlock)
	if !ok {
		return "", trace.Errorf("invalid block type %T", block)
	}

	elementItems := contextBlock.ElementItems
	if n := len(elementItems); n != 1 {
		return "", trace.Errorf("expected only one context element, got %v", n)
	}

	element := elementItems[0].ContextElement
	textBlock, ok := element.(TextObject)
	if !ok {
		return "", trace.Errorf("invalid element type %T", element)
	}

	return textBlock.GetText(), nil
}
