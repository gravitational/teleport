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

package msteams

import (
	"context"
	"os/user"
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
	"github.com/tidwall/gjson"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/msteams/msapi"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
)

type TeamsSuite struct {
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
	mockAPI    *MockMSTeamsAPI

	clients          map[string]*integration.Client
	teleportFeatures *proto.Features
	teleportConfig   lib.TeleportConfig
}

func TestMSTeams(t *testing.T) { suite.Run(t, &TeamsSuite{}) }

func (s *TeamsSuite) SetupSuite() {
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

	role, err = bootstrap.AddRole("access-msteams", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule("access_request", []string{"list", "read"}),
				types.NewRule("access_plugin_data", []string{"update"}),
			},
		},
	})
	require.NoError(t, err)

	user, err = bootstrap.AddUserWithRoles("access-msteams", role.GetName())
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

func (s *TeamsSuite) SetupTest() {
	t := s.T()

	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)

	s.mockAPI = NewMockMSTeamsAPI(s.raceNumber)
	t.Cleanup(s.mockAPI.Close)

	var conf Config
	conf.Teleport = s.teleportConfig
	conf.MSAPI = s.mockAPI.Config
	conf.MSAPI.SetBaseURLs(s.mockAPI.URL(), s.mockAPI.URL(), s.mockAPI.URL())

	s.appConfig = conf
	s.SetContextTimeout(5 * time.Second)
}

func (s *TeamsSuite) startApp() {
	t := s.T()
	t.Helper()

	app, err := NewApp(s.appConfig)
	require.NoError(t, err)

	s.StartApp(app)
}

func (s *TeamsSuite) ruler() *integration.Client {
	return s.clients[s.userNames.ruler]
}

func (s *TeamsSuite) requestor() *integration.Client {
	return s.clients[s.userNames.requestor]
}

func (s *TeamsSuite) reviewer1() *integration.Client {
	return s.clients[s.userNames.reviewer1]
}

func (s *TeamsSuite) reviewer2() *integration.Client {
	return s.clients[s.userNames.reviewer2]
}

func (s *TeamsSuite) newAccessRequest(reviewers []msapi.User) types.AccessRequest {
	t := s.T()
	t.Helper()

	req, err := types.NewAccessRequest(uuid.New().String(), s.userNames.requestor, "editor")
	require.NoError(t, err)
	req.SetRequestReason("because of " + strings.Repeat("A", 4000))

	var suggestedReviewers []string
	for _, user := range reviewers {
		suggestedReviewers = append(suggestedReviewers, user.Mail)
	}
	req.SetSuggestedReviewers(suggestedReviewers)

	return req
}

func (s *TeamsSuite) createAccessRequest(reviewers []msapi.User) types.AccessRequest {
	t := s.T()
	t.Helper()

	req := s.newAccessRequest(reviewers)
	err := s.requestor().CreateAccessRequest(s.Context(), req)
	require.NoError(t, err)
	return req
}

func (s *TeamsSuite) checkPluginData(reqID string, cond func(interface{}) bool) interface{} {
	t := s.T()
	t.Helper()

	for {
		rawData, err := s.ruler().PollAccessRequestPluginData(s.Context(), "msteams", reqID)
		require.NoError(t, err)
		data, err := DecodePluginData(rawData)
		require.NoError(t, err)
		if cond(data) {
			return data
		}
	}
}

func (s *TeamsSuite) getNewMessages(t *testing.T, n int) MsgSlice {
	msgs := MsgSlice{}
	for i := 0; i < 2; i++ {
		msg, err := s.mockAPI.CheckNewMessage(s.Context())
		require.NoError(t, err)
		msgs = append(msgs, msg)
	}
	sort.Sort(msgs)
	return msgs
}

func (s *TeamsSuite) TestMessagePosting() {
	t := s.T()

	reviewer1 := s.mockAPI.StoreUser(msapi.User{Mail: s.userNames.reviewer1})
	reviewer2 := s.mockAPI.StoreUser(msapi.User{Mail: s.userNames.reviewer2})

	s.startApp()

	request := s.createAccessRequest([]msapi.User{reviewer2, reviewer1})

	pluginData := s.checkPluginData(request.GetName(), func(data interface{}) bool {
		return len(data.(PluginData).TeamsData) > 0
	})
	require.Len(t, pluginData.(PluginData).TeamsData, 2)

	title := "Access Request " + request.GetName()

	msgs := s.getNewMessages(t, 2)

	require.Equal(t, gjson.Get(msgs[0].Body, "attachments.0.content.body.0.text").String(), title)
	require.Equal(t, msgs[0].RecipientID, reviewer1.ID)

	require.Equal(t, gjson.Get(msgs[1].Body, "attachments.0.content.body.0.text").String(), title)
	require.Equal(t, msgs[1].RecipientID, reviewer2.ID)
}

func (s *TeamsSuite) TestRecipientsConfig() {
	t := s.T()

	reviewer1 := s.mockAPI.StoreUser(msapi.User{Mail: s.userNames.reviewer1})
	reviewer2 := s.mockAPI.StoreUser(msapi.User{Mail: s.userNames.reviewer2})
	s.appConfig.Recipients = common.RawRecipientsMap{
		types.Wildcard: []string{reviewer2.Mail, reviewer1.ID},
	}

	s.startApp()

	request := s.createAccessRequest(nil)
	pluginData := s.checkPluginData(request.GetName(), func(data interface{}) bool {
		return len(data.(PluginData).TeamsData) > 0
	})
	require.Len(t, pluginData.(PluginData).TeamsData, 2)

	title := "Access Request " + request.GetName()

	msgs := s.getNewMessages(t, 2)

	require.Equal(t, gjson.Get(msgs[0].Body, "attachments.0.content.body.0.text").String(), title)
	require.Equal(t, msgs[0].RecipientID, reviewer1.ID)

	require.Equal(t, gjson.Get(msgs[1].Body, "attachments.0.content.body.0.text").String(), title)
	require.Equal(t, msgs[1].RecipientID, reviewer2.ID)
}

func (s *TeamsSuite) TestApproval() {
	t := s.T()

	reviewer := s.mockAPI.StoreUser(msapi.User{Mail: s.userNames.reviewer1})

	s.startApp()

	req := s.createAccessRequest([]msapi.User{reviewer})
	msg, err := s.mockAPI.CheckNewMessage(s.Context())
	require.NoError(t, err)
	require.Equal(t, reviewer.ID, msg.RecipientID)

	err = s.ruler().ApproveAccessRequest(s.Context(), req.GetName(), "okay")
	require.NoError(t, err)

	msgUpdate, err := s.mockAPI.CheckMessageUpdate(s.Context())
	require.NoError(t, err)

	require.Equal(t, reviewer.ID, msgUpdate.RecipientID)
	require.Equal(t, msg.ID, msgUpdate.ID)

	require.NoError(t, err)
	require.Equal(t, gjson.Get(msgUpdate.Body, "attachments.0.content.body.1.columns.0.items.0.text").String(), "✅")
	require.Equal(t, gjson.Get(msgUpdate.Body, "attachments.0.content.body.1.columns.1.items.0.text").String(), "APPROVED")
	require.Equal(t, gjson.Get(msgUpdate.Body, "attachments.0.content.body.2.facts.4.value").String(), "okay")
}

func (s *TeamsSuite) TestDenial() {
	t := s.T()

	reviewer := s.mockAPI.StoreUser(msapi.User{Mail: s.userNames.reviewer1})

	s.startApp()

	req := s.createAccessRequest([]msapi.User{reviewer})
	msg, err := s.mockAPI.CheckNewMessage(s.Context())
	require.NoError(t, err)
	require.Equal(t, reviewer.ID, msg.RecipientID)

	// max size of request was decreased here: https://github.com/gravitational/teleport/pull/13298
	err = s.ruler().DenyAccessRequest(s.Context(), req.GetName(), "not okay")
	require.NoError(t, err)

	msgUpdate, err := s.mockAPI.CheckMessageUpdate(s.Context())
	require.NoError(t, err)

	require.Equal(t, reviewer.ID, msgUpdate.RecipientID)
	require.Equal(t, msg.ID, msgUpdate.ID)

	require.NoError(t, err)
	require.Equal(t, gjson.Get(msgUpdate.Body, "attachments.0.content.body.1.columns.0.items.0.text").String(), "❌")
	require.Equal(t, gjson.Get(msgUpdate.Body, "attachments.0.content.body.1.columns.1.items.0.text").String(), "DENIED")
	require.Equal(t, gjson.Get(msgUpdate.Body, "attachments.0.content.body.2.facts.4.value").String(), "not okay")
}

func (s *TeamsSuite) TestReviewReplies() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	reviewer := s.mockAPI.StoreUser(msapi.User{Mail: s.userNames.reviewer1})

	s.startApp()

	req := s.createAccessRequest([]msapi.User{reviewer})
	s.checkPluginData(req.GetName(), func(data interface{}) bool {
		return len(data.(PluginData).TeamsData) > 0
	})

	msg, err := s.mockAPI.CheckNewMessage(s.Context())
	require.NoError(t, err)
	require.Equal(t, reviewer.ID, msg.RecipientID)

	err = s.reviewer1().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer1,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	reply, err := s.mockAPI.CheckMessageUpdate(s.Context())
	require.NoError(t, err)

	require.Equal(t, msg.RecipientID, reply.RecipientID)
	require.Equal(t, msg.ID, reply.ID)
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.4.facts.0.value").String(), "✅")
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.4.facts.1.value").String(), s.userNames.reviewer1)
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.4.facts.3.value").String(), "okay")

	err = s.reviewer2().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer2,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	reply, err = s.mockAPI.CheckMessageUpdate(s.Context())
	require.NoError(t, err)

	require.Equal(t, msg.RecipientID, reply.RecipientID)
	require.Equal(t, msg.ID, reply.ID)
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.5.facts.0.value").String(), "❌")
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.5.facts.1.value").String(), s.userNames.reviewer2)
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.5.facts.3.value").String(), "not okay")
}

func (s *TeamsSuite) TestApprovalByReview() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	reviewer := s.mockAPI.StoreUser(msapi.User{Mail: s.userNames.reviewer1})

	s.startApp()

	req := s.createAccessRequest([]msapi.User{reviewer})
	msg, err := s.mockAPI.CheckNewMessage(s.Context())
	require.NoError(t, err)
	require.Equal(t, reviewer.ID, msg.RecipientID)

	err = s.reviewer1().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer1,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	reply, err := s.mockAPI.CheckMessageUpdate(s.Context())
	require.NoError(t, err)

	require.Equal(t, msg.RecipientID, reply.RecipientID)
	require.Equal(t, msg.ID, reply.ID)
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.4.facts.0.value").String(), "✅")
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.4.facts.1.value").String(), s.userNames.reviewer1)
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.4.facts.3.value").String(), "okay")

	err = s.reviewer2().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer2,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "finally okay",
	})
	require.NoError(t, err)

	reply, err = s.mockAPI.CheckMessageUpdate(s.Context())
	require.NoError(t, err)

	require.Equal(t, msg.RecipientID, reply.RecipientID)
	require.Equal(t, msg.ID, reply.ID)
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.5.facts.0.value").String(), "✅")
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.5.facts.1.value").String(), s.userNames.reviewer2)
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.5.facts.3.value").String(), "finally okay")
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.1.columns.0.items.0.text").String(), "✅")
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.1.columns.1.items.0.text").String(), "APPROVED")
}

func (s *TeamsSuite) TestDenialByReview() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	reviewer := s.mockAPI.StoreUser(msapi.User{Mail: s.userNames.reviewer1})

	s.startApp()

	req := s.createAccessRequest([]msapi.User{reviewer})
	msg, err := s.mockAPI.CheckNewMessage(s.Context())
	require.NoError(t, err)
	require.Equal(t, reviewer.ID, msg.RecipientID)

	err = s.reviewer1().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer1,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	reply, err := s.mockAPI.CheckMessageUpdate(s.Context())
	require.NoError(t, err)

	require.Equal(t, msg.RecipientID, reply.RecipientID)
	require.Equal(t, msg.ID, reply.ID)
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.4.facts.0.value").String(), "❌")
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.4.facts.1.value").String(), s.userNames.reviewer1)
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.4.facts.3.value").String(), "not okay")

	err = s.reviewer2().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer2,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "finally not okay",
	})
	require.NoError(t, err)

	reply, err = s.mockAPI.CheckMessageUpdate(s.Context())
	require.NoError(t, err)

	require.Equal(t, msg.RecipientID, reply.RecipientID)
	require.Equal(t, msg.ID, reply.ID)
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.5.facts.0.value").String(), "❌")
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.5.facts.1.value").String(), s.userNames.reviewer2)
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.5.facts.3.value").String(), "finally not okay")
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.1.columns.0.items.0.text").String(), "❌")
	require.Equal(t, gjson.Get(reply.Body, "attachments.0.content.body.1.columns.1.items.0.text").String(), "DENIED")
}

func (s *TeamsSuite) TestExpiration() {
	t := s.T()

	reviewer := s.mockAPI.StoreUser(msapi.User{Mail: s.userNames.reviewer1})

	s.startApp()

	request := s.createAccessRequest([]msapi.User{reviewer})
	msg, err := s.mockAPI.CheckNewMessage(s.Context())
	require.NoError(t, err)
	require.Equal(t, reviewer.ID, msg.RecipientID)

	s.checkPluginData(request.GetName(), func(data interface{}) bool {
		return len(data.(PluginData).TeamsData) > 0
	})

	err = s.ruler().DeleteAccessRequest(s.Context(), request.GetName()) // simulate expiration
	require.NoError(t, err)

	msgUpdate, err := s.mockAPI.CheckMessageUpdate(s.Context())
	require.NoError(t, err)
	require.Equal(t, reviewer.ID, msgUpdate.RecipientID)
	require.Equal(t, msg.ID, msgUpdate.ID)

	require.Equal(t, gjson.Get(msgUpdate.Body, "attachments.0.content.body.1.columns.0.items.0.text").String(), "⌛")
	require.Equal(t, gjson.Get(msgUpdate.Body, "attachments.0.content.body.1.columns.1.items.0.text").String(), "EXPIRED")
}

func (s *TeamsSuite) TestRace() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	err := logger.Setup(logger.Config{Severity: "info"}) // Turn off noisy debug logging
	require.NoError(t, err)

	reviewer1 := s.mockAPI.StoreUser(msapi.User{Mail: s.userNames.reviewer1})
	reviewer2 := s.mockAPI.StoreUser(msapi.User{Mail: s.userNames.reviewer2})

	s.SetContextTimeout(20 * time.Second)
	s.startApp()

	var (
		raceErr           error
		raceErrOnce       sync.Once
		msgIDs            sync.Map
		msgsCount         int32
		msgUpdateCounters sync.Map
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
			req.SetSuggestedReviewers([]string{reviewer1.Mail, reviewer2.Mail})
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
	for i := 0; i < 2*s.raceNumber; i++ {
		process.SpawnCritical(func(ctx context.Context) error {
			msg, err := s.mockAPI.CheckNewMessage(ctx)
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}

			threadMsgKey := TeamsMessage{ID: msg.ID, RecipientID: msg.RecipientID}
			if _, loaded := msgIDs.LoadOrStore(threadMsgKey, struct{}{}); loaded {
				return setRaceErr(trace.Errorf("thread %v already stored", threadMsgKey))
			}
			atomic.AddInt32(&msgsCount, 1)

			user, ok := s.mockAPI.GetUser(msg.RecipientID)
			if !ok {
				return setRaceErr(trace.Errorf("user %s is not found", msg.RecipientID))
			}

			title := gjson.Get(msg.Body, "attachments.0.content.body.0.text").String()
			reqID := title[strings.LastIndex(title, " ")+1:]

			if err = s.clients[user.Mail].SubmitAccessRequestReview(ctx, reqID, types.AccessReview{
				Author:        user.Mail,
				ProposedState: types.RequestState_APPROVED,
				Created:       time.Now(),
				Reason:        "okay",
			}); err != nil {
				return setRaceErr(trace.Wrap(err))
			}

			return nil
		})
	}

	// Multiplier TWO means that we handle updates for each of the two messages posted to reviewers.
	for i := 0; i < 4*s.raceNumber; i++ {
		process.SpawnCritical(func(ctx context.Context) error {
			msg, err := s.mockAPI.CheckMessageUpdate(ctx)
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}

			threadMsgKey := TeamsMessage{ID: msg.ID, RecipientID: msg.RecipientID}
			var newCounter int32
			val, _ := msgUpdateCounters.LoadOrStore(threadMsgKey, &newCounter)
			counterPtr := val.(*int32)
			atomic.AddInt32(counterPtr, 1)

			return nil
		})
	}

	time.Sleep(1 * time.Second)

	process.Terminate()
	<-process.Done()
	require.NoError(t, raceErr)

	require.Equal(t, int32(2*s.raceNumber), msgsCount)
	msgIDs.Range(func(key, value interface{}) bool {
		next := true

		val, loaded := msgUpdateCounters.LoadAndDelete(key)
		next = next && assert.True(t, loaded)
		counterPtr := val.(*int32)
		next = next && assert.Equal(t, int32(2), *counterPtr)

		return next
	})
}
