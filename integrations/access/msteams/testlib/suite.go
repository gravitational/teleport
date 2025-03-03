// Copyright 2024 Gravitational, Inc
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

package testlib

import (
	"context"
	"encoding/json"
	"runtime"
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
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/msteams"
	"github.com/gravitational/teleport/integrations/access/msteams/msapi"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
)

// MsTeamsBaseSuite is the MsTeams access plugin test suite.
// It implements the testify.TestingSuite interface.
type MsTeamsBaseSuite struct {
	*integration.AccessRequestSuite
	appConfig             *msteams.Config
	raceNumber            int
	fakeTeams             *FakeTeams
	fakeStatusSink        *integration.FakeStatusSink
	requester1TeamsUser   msapi.User
	requesterOSSTeamsUser msapi.User
	reviewer1TeamsUser    msapi.User
	reviewer2TeamsUser    msapi.User
}

// SetupTest starts a fake MsTeams, generates the plugin configuration, and loads
// the fixtures in MsTeams. It runs for each test.
func (s *MsTeamsBaseSuite) SetupTest() {
	t := s.T()

	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)
	s.raceNumber = runtime.GOMAXPROCS(0)

	s.fakeTeams = NewFakeTeams(s.raceNumber)
	t.Cleanup(s.fakeTeams.Close)

	// We need requester users as well, the MsTeams plugin sends messages to users
	// when their access request got approved.
	s.requesterOSSTeamsUser = s.fakeTeams.StoreUser(msapi.User{Name: "Requester OSS", Mail: integration.RequesterOSSUserName})
	s.requester1TeamsUser = s.fakeTeams.StoreUser(msapi.User{Name: "Requester Ent", Mail: integration.Requester1UserName})
	s.reviewer1TeamsUser = s.fakeTeams.StoreUser(msapi.User{Mail: integration.Reviewer1UserName})
	s.reviewer2TeamsUser = s.fakeTeams.StoreUser(msapi.User{Mail: integration.Reviewer2UserName})

	s.fakeStatusSink = &integration.FakeStatusSink{}

	var conf msteams.Config
	conf.Teleport = s.TeleportConfig()
	apiClient, err := common.GetTeleportClient(context.Background(), s.TeleportConfig())
	require.NoError(t, err)
	conf.Client = apiClient
	conf.StatusSink = s.fakeStatusSink
	conf.MSAPI = s.fakeTeams.Config
	conf.MSAPI.SetBaseURLs(s.fakeTeams.URL(), s.fakeTeams.URL(), s.fakeTeams.URL())

	s.appConfig = &conf
}

// startApp starts the MsTeams plugin, waits for it to become ready and returns.
func (s *MsTeamsBaseSuite) startApp() {
	s.T().Helper()
	t := s.T()

	app, err := msteams.NewApp(*s.appConfig)
	require.NoError(t, err)
	integration.RunAndWaitReady(t, app)
}

// MsTeamsSuiteOSS contains all tests that support running against a Teleport
// OSS Server.
type MsTeamsSuiteOSS struct {
	MsTeamsBaseSuite
}

// MsTeamsSuiteEnterprise contains all tests that require a Teleport Enterprise
// to run.
type MsTeamsSuiteEnterprise struct {
	MsTeamsBaseSuite
}

// SetupTest overrides MsTeamsBaseSuite.SetupTest to check the Teleport features
// before each test.
func (s *MsTeamsSuiteEnterprise) SetupTest() {
	t := s.T()
	s.RequireAdvancedWorkflow(t)
	s.MsTeamsBaseSuite.SetupTest()
}

func (s *MsTeamsSuiteOSS) TestMessagePosting() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	request := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, []string{s.reviewer1TeamsUser.Mail, s.reviewer2TeamsUser.Mail})

	pluginData := s.checkPluginData(ctx, request.GetName(), func(data msteams.PluginData) bool {
		return len(data.TeamsData) > 0
	})
	require.Len(t, pluginData.TeamsData, 2)

	title := "Access Request " + request.GetName()

	msgs, err := s.getNewMessages(ctx, 2)
	require.NoError(t, err)

	var body1 testTeamsMessage
	require.NoError(t, json.Unmarshal([]byte(msgs[0].Body), &body1))
	body1.checkTitle(t, title)
	require.Equal(t, msgs[0].RecipientID, s.reviewer1TeamsUser.ID)

	var body2 testTeamsMessage
	require.NoError(t, json.Unmarshal([]byte(msgs[1].Body), &body2))
	body1.checkTitle(t, title)
	require.Equal(t, msgs[1].RecipientID, s.reviewer2TeamsUser.ID)
}

func (s *MsTeamsSuiteOSS) TestRecipientsConfig() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.appConfig.Recipients = common.RawRecipientsMap{
		types.Wildcard: []string{s.reviewer2TeamsUser.Mail, s.reviewer1TeamsUser.ID},
	}

	s.startApp()

	request := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)
	pluginData := s.checkPluginData(ctx, request.GetName(), func(data msteams.PluginData) bool {
		return len(data.TeamsData) > 0
	})
	require.Len(t, pluginData.TeamsData, 2)

	title := "Access Request " + request.GetName()

	msgs, err := s.getNewMessages(ctx, 2)
	require.NoError(t, err)

	var body1 testTeamsMessage
	require.NoError(t, json.Unmarshal([]byte(msgs[0].Body), &body1))
	body1.checkTitle(t, title)
	require.Equal(t, msgs[0].RecipientID, s.reviewer1TeamsUser.ID)

	var body2 testTeamsMessage
	require.NoError(t, json.Unmarshal([]byte(msgs[1].Body), &body2))
	body1.checkTitle(t, title)
	require.Equal(t, msgs[1].RecipientID, s.reviewer2TeamsUser.ID)
}

func (s *MsTeamsSuiteOSS) TestApproval() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, []string{s.reviewer1TeamsUser.Mail})
	msg, err := s.fakeTeams.CheckNewMessage(ctx)
	require.NoError(t, err)
	require.Equal(t, s.reviewer1TeamsUser.ID, msg.RecipientID)

	const reason = "okay"
	err = s.Ruler().ApproveAccessRequest(ctx, req.GetName(), reason)
	require.NoError(t, err)

	msgUpdate, err := s.fakeTeams.CheckMessageUpdate(ctx)
	require.NoError(t, err)

	require.Equal(t, s.reviewer1TeamsUser.ID, msg.RecipientID)
	require.Equal(t, msg.ID, msgUpdate.ID)

	require.NoError(t, err)
	var body testTeamsMessage
	require.NoError(t, json.Unmarshal([]byte(msgUpdate.Body), &body))
	body.checkStatusApproved(t, reason)
}

func (s *MsTeamsSuiteOSS) TestDenial() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, []string{s.reviewer1TeamsUser.Mail})
	msg, err := s.fakeTeams.CheckNewMessage(ctx)
	require.NoError(t, err)
	require.Equal(t, s.reviewer1TeamsUser.ID, msg.RecipientID)

	const reason = "not okay"
	err = s.Ruler().DenyAccessRequest(ctx, req.GetName(), reason)
	require.NoError(t, err)

	msgUpdate, err := s.fakeTeams.CheckMessageUpdate(ctx)
	require.NoError(t, err)

	require.Equal(t, s.reviewer1TeamsUser.ID, msg.RecipientID)
	require.Equal(t, msg.ID, msgUpdate.ID)

	require.NoError(t, err)
	var body testTeamsMessage
	require.NoError(t, json.Unmarshal([]byte(msgUpdate.Body), &body))
	body.checkStatusDenied(t, reason)
}

func (s *MsTeamsSuiteEnterprise) TestReviewReplies() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	req := s.CreateAccessRequest(ctx, integration.Requester1UserName, []string{s.reviewer1TeamsUser.Mail})
	s.checkPluginData(ctx, req.GetName(), func(data msteams.PluginData) bool {
		return len(data.TeamsData) > 0
	})

	msg, err := s.fakeTeams.CheckNewMessage(ctx)
	require.NoError(t, err)
	require.Equal(t, s.reviewer1TeamsUser.ID, msg.RecipientID)

	err = s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	reply, err := s.fakeTeams.CheckMessageUpdate(ctx)
	require.NoError(t, err)

	require.Equal(t, msg.RecipientID, reply.RecipientID)
	require.Equal(t, msg.ID, reply.ID)
	var body testTeamsMessage
	require.NoError(t, json.Unmarshal([]byte(reply.Body), &body))
	body.checkReview(t, 0, true /* approved */, "okay", integration.Reviewer1UserName)

	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	reply, err = s.fakeTeams.CheckMessageUpdate(ctx)
	require.NoError(t, err)

	require.Equal(t, msg.RecipientID, reply.RecipientID)
	require.Equal(t, msg.ID, reply.ID)
	require.NoError(t, json.Unmarshal([]byte(reply.Body), &body))
	body.checkReview(t, 1, false /* approved */, "not okay", integration.Reviewer2UserName)
}

func (s *MsTeamsSuiteEnterprise) TestApprovalByReview() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	req := s.CreateAccessRequest(ctx, integration.Requester1UserName, []string{s.reviewer1TeamsUser.Mail})
	s.checkPluginData(ctx, req.GetName(), func(data msteams.PluginData) bool {
		return len(data.TeamsData) > 0
	})

	msg, err := s.fakeTeams.CheckNewMessage(ctx)
	require.NoError(t, err)
	require.Equal(t, s.reviewer1TeamsUser.ID, msg.RecipientID)

	err = s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	reply, err := s.fakeTeams.CheckMessageUpdate(ctx)
	require.NoError(t, err)

	require.Equal(t, msg.RecipientID, reply.RecipientID)
	require.Equal(t, msg.ID, reply.ID)
	var body testTeamsMessage
	require.NoError(t, json.Unmarshal([]byte(reply.Body), &body))
	body.checkReview(t, 0, true /* approved */, "okay", integration.Reviewer1UserName)

	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "finally okay",
	})
	require.NoError(t, err)

	reply, err = s.fakeTeams.CheckMessageUpdate(ctx)
	require.NoError(t, err)

	require.Equal(t, msg.RecipientID, reply.RecipientID)
	require.Equal(t, msg.ID, reply.ID)
	require.NoError(t, json.Unmarshal([]byte(reply.Body), &body))
	body.checkReview(t, 1, true /* approved */, "finally okay", integration.Reviewer2UserName)
	body.checkStatusApproved(t, "finally okay")
}

func (s *MsTeamsSuiteEnterprise) TestDenialByReview() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	req := s.CreateAccessRequest(ctx, integration.Requester1UserName, []string{s.reviewer1TeamsUser.Mail})
	s.checkPluginData(ctx, req.GetName(), func(data msteams.PluginData) bool {
		return len(data.TeamsData) > 0
	})

	msg, err := s.fakeTeams.CheckNewMessage(ctx)
	require.NoError(t, err)
	require.Equal(t, s.reviewer1TeamsUser.ID, msg.RecipientID)

	err = s.Reviewer1().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer1UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	reply, err := s.fakeTeams.CheckMessageUpdate(ctx)
	require.NoError(t, err)

	require.Equal(t, msg.RecipientID, reply.RecipientID)
	require.Equal(t, msg.ID, reply.ID)
	var body testTeamsMessage
	require.NoError(t, json.Unmarshal([]byte(reply.Body), &body))
	body.checkReview(t, 0, false /* approved */, "not okay", integration.Reviewer1UserName)

	err = s.Reviewer2().SubmitAccessRequestReview(ctx, req.GetName(), types.AccessReview{
		Author:        integration.Reviewer2UserName,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "finally not okay",
	})
	require.NoError(t, err)

	reply, err = s.fakeTeams.CheckMessageUpdate(ctx)
	require.NoError(t, err)

	require.Equal(t, msg.RecipientID, reply.RecipientID)
	require.Equal(t, msg.ID, reply.ID)
	require.NoError(t, json.Unmarshal([]byte(reply.Body), &body))
	body.checkReview(t, 1, false /* approved */, "finally not okay", integration.Reviewer2UserName)
	body.checkStatusDenied(t, "finally not okay")
}

func (s *MsTeamsSuiteOSS) TestExpiration() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	request := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, []string{s.reviewer1TeamsUser.Mail})
	msg, err := s.fakeTeams.CheckNewMessage(ctx)
	require.NoError(t, err)
	require.Equal(t, s.reviewer1TeamsUser.ID, msg.RecipientID)

	s.checkPluginData(ctx, request.GetName(), func(data msteams.PluginData) bool {
		return len(data.TeamsData) > 0
	})

	err = s.Ruler().DeleteAccessRequest(ctx, request.GetName()) // simulate expiration
	require.NoError(t, err)

	msgUpdate, err := s.fakeTeams.CheckMessageUpdate(ctx)
	require.NoError(t, err)
	require.Equal(t, s.reviewer1TeamsUser.ID, msgUpdate.RecipientID)
	require.Equal(t, msg.ID, msgUpdate.ID)

	var body testTeamsMessage
	require.NoError(t, json.Unmarshal([]byte(msgUpdate.Body), &body))
	body.checkStatusExpired(t)
}
func (s *MsTeamsSuiteEnterprise) TestRace() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	err := logger.Setup(logger.Config{Severity: "info"}) // Turn off noisy debug logging
	require.NoError(t, err)

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

	process := lib.NewProcess(ctx)
	for i := 0; i < s.raceNumber; i++ {
		process.SpawnCritical(func(ctx context.Context) error {
			req, err := types.NewAccessRequest(uuid.New().String(), integration.Requester1UserName, "editor")
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			req.SetSuggestedReviewers([]string{s.reviewer1TeamsUser.Mail, s.reviewer2TeamsUser.Mail})
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
	for i := 0; i < 2*s.raceNumber; i++ {
		process.SpawnCritical(func(ctx context.Context) error {
			msg, err := s.fakeTeams.CheckNewMessage(ctx)
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}

			threadMsgKey := msteams.TeamsMessage{ID: msg.ID, RecipientID: msg.RecipientID}
			if _, loaded := msgIDs.LoadOrStore(threadMsgKey, struct{}{}); loaded {
				return setRaceErr(trace.Errorf("thread %v already stored", threadMsgKey))
			}
			atomic.AddInt32(&msgsCount, 1)

			user, ok := s.fakeTeams.GetUser(msg.RecipientID)
			if !ok {
				return setRaceErr(trace.Errorf("user %s is not found", msg.RecipientID))
			}

			var body testTeamsMessage
			err = json.Unmarshal([]byte(msg.Body), &body)
			if err != nil {
				return setRaceErr(trace.Wrap(err, "unmarshalling message"))
			}
			title := body.getTitle()
			reqID := title[strings.LastIndex(title, " ")+1:]

			if err = s.ClientByName(user.Mail).SubmitAccessRequestReview(ctx, reqID, types.AccessReview{
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
			msg, err := s.fakeTeams.CheckMessageUpdate(ctx)
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}

			threadMsgKey := msteams.TeamsMessage{ID: msg.ID, RecipientID: msg.RecipientID}
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

func (s *MsTeamsSuiteOSS) TestRecipientsFromAccessMonitoringRule() {
	t := s.T()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	s.startApp()

	_, err := s.ClientByName(integration.RulerUserName).
		AccessMonitoringRulesClient().
		CreateAccessMonitoringRule(ctx, &accessmonitoringrulesv1.AccessMonitoringRule{
			Kind:    types.KindAccessMonitoringRule,
			Version: types.V1,
			Metadata: &v1.Metadata{
				Name: "test-msteams-amr",
			},
			Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
				Subjects:  []string{types.KindAccessRequest},
				Condition: "!is_empty(access_request.spec.roles)",
				Notification: &accessmonitoringrulesv1.Notification{
					Name: "msteams",
					Recipients: []string{
						s.reviewer1TeamsUser.ID,
						s.reviewer2TeamsUser.Mail,
					},
				},
			},
		})
	assert.NoError(t, err)

	// Test execution: create an access request
	req := s.CreateAccessRequest(ctx, integration.RequesterOSSUserName, nil)

	s.checkPluginData(ctx, req.GetName(), func(data msteams.PluginData) bool {
		return len(data.TeamsData) > 0
	})

	title := "Access Request " + req.GetName()
	msgs, err := s.getNewMessages(ctx, 2)
	require.NoError(t, err)

	var body1 testTeamsMessage
	require.NoError(t, json.Unmarshal([]byte(msgs[0].Body), &body1))
	body1.checkTitle(t, title)
	require.Equal(t, msgs[0].RecipientID, s.reviewer1TeamsUser.ID)

	var body2 testTeamsMessage
	require.NoError(t, json.Unmarshal([]byte(msgs[1].Body), &body2))
	body1.checkTitle(t, title)
	require.Equal(t, msgs[1].RecipientID, s.reviewer2TeamsUser.ID)

	assert.NoError(t, s.ClientByName(integration.RulerUserName).
		AccessMonitoringRulesClient().DeleteAccessMonitoringRule(ctx, "test-msteams-amr"))
}
