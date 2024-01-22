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

package mattermost

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

var msgFieldRegexp = regexp.MustCompile(`(?im)^\*\*([a-zA-Z ]+)\*\*:\ +(.+)$`)
var requestReasonRegexp = regexp.MustCompile("(?im)^\\*\\*Reason\\*\\*:\\ ```\\n(.*?)```(.*?)$")
var resolutionReasonRegexp = regexp.MustCompile("(?im)^\\*\\*Resolution reason\\*\\*:\\ ```\\n(.*?)```(.*?)$")

type MattermostSuite struct {
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
	fakeMattermost *FakeMattermost
	fakeStatusSink *fakeStatusSink
	mmUser         User

	clients          map[string]*integration.Client
	teleportFeatures *proto.Features
	teleportConfig   lib.TeleportConfig
}

func TestMattermost(t *testing.T) { suite.Run(t, &MattermostSuite{}) }

func (s *MattermostSuite) SetupSuite() {
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

	role, err = bootstrap.AddRole("access-mattermost", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule("access_request", []string{"list", "read"}),
				types.NewRule("access_plugin_data", []string{"update"}),
			},
		},
	})
	require.NoError(t, err)

	user, err = bootstrap.AddUserWithRoles("access-mattermost", role.GetName())
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

func (s *MattermostSuite) SetupTest() {
	t := s.T()

	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)

	s.fakeMattermost = NewFakeMattermost(User{Username: "bot", Email: "bot@example.com"}, s.raceNumber)
	t.Cleanup(s.fakeMattermost.Close)

	s.mmUser = s.fakeMattermost.StoreUser(User{
		FirstName: "User",
		LastName:  "Test",
		Username:  "Vladimir",
		Email:     s.userNames.requestor,
	})

	s.fakeStatusSink = &fakeStatusSink{}

	var conf Config
	conf.Teleport = s.teleportConfig
	conf.Mattermost.Token = "000000"
	conf.Mattermost.URL = s.fakeMattermost.URL()
	conf.StatusSink = s.fakeStatusSink

	s.appConfig = &conf
	s.SetContextTimeout(5 * time.Second)
}

func (s *MattermostSuite) startApp() {
	t := s.T()
	t.Helper()

	app := NewMattermostApp(s.appConfig)
	s.StartApp(app)
}

func (s *MattermostSuite) ruler() *integration.Client {
	return s.clients[s.userNames.ruler]
}

func (s *MattermostSuite) requestor() *integration.Client {
	return s.clients[s.userNames.requestor]
}

func (s *MattermostSuite) reviewer1() *integration.Client {
	return s.clients[s.userNames.reviewer1]
}

func (s *MattermostSuite) reviewer2() *integration.Client {
	return s.clients[s.userNames.reviewer2]
}

func (s *MattermostSuite) newAccessRequest(reviewers []User) types.AccessRequest {
	t := s.T()
	t.Helper()

	req, err := types.NewAccessRequest(uuid.New().String(), s.userNames.requestor, "editor")
	require.NoError(t, err)
	// max size of request was decreased here: https://github.com/gravitational/teleport/pull/13298
	req.SetRequestReason("because of " + strings.Repeat("A", 4000))
	var suggestedReviewers []string
	for _, user := range reviewers {
		suggestedReviewers = append(suggestedReviewers, user.Email)
	}
	req.SetSuggestedReviewers(suggestedReviewers)
	return req
}

func (s *MattermostSuite) createAccessRequest(reviewers []User) types.AccessRequest {
	t := s.T()
	t.Helper()

	req := s.newAccessRequest(reviewers)
	out, err := s.requestor().CreateAccessRequestV2(s.Context(), req)
	require.NoError(s.T(), err)
	return out
}

func (s *MattermostSuite) checkPluginData(reqID string, cond func(accessrequest.PluginData) bool) accessrequest.PluginData {
	t := s.T()
	t.Helper()

	for {
		rawData, err := s.ruler().PollAccessRequestPluginData(s.Context(), "mattermost", reqID)
		require.NoError(t, err)
		data, err := accessrequest.DecodePluginData(rawData)
		require.NoError(t, err)
		if cond(data) {
			return data
		}
	}
}

func (s *MattermostSuite) TestMattermostMessagePosting() {
	t := s.T()

	reviewer1 := s.fakeMattermost.StoreUser(User{Email: s.userNames.reviewer1})
	reviewer2 := s.fakeMattermost.StoreUser(User{Email: s.userNames.reviewer2})
	directChannel1 := s.fakeMattermost.GetDirectChannelFor(s.fakeMattermost.GetBotUser(), reviewer1)
	directChannel2 := s.fakeMattermost.GetDirectChannelFor(s.fakeMattermost.GetBotUser(), reviewer2)

	s.startApp()
	request := s.createAccessRequest([]User{reviewer2, reviewer1})

	pluginData := s.checkPluginData(request.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})
	assert.Len(t, pluginData.SentMessages, 2)

	var posts []Post
	postSet := make(MattermostDataPostSet)
	for i := 0; i < 2; i++ {
		post, err := s.fakeMattermost.CheckNewPost(s.Context())
		require.NoError(t, err, "no new messages posted")
		postSet.Add(accessrequest.MessageData{ChannelID: post.ChannelID, MessageID: post.ID})
		posts = append(posts, post)
	}

	assert.Len(t, postSet, 2)
	assert.Contains(t, postSet, pluginData.SentMessages[0])
	assert.Contains(t, postSet, pluginData.SentMessages[1])

	sort.Sort(MattermostPostSlice(posts))

	assert.Equal(t, directChannel1.ID, posts[0].ChannelID)
	assert.Equal(t, directChannel2.ID, posts[1].ChannelID)

	post := posts[0]
	reqID, err := parsePostField(post, "Request ID")
	require.NoError(t, err)
	assert.Equal(t, request.GetName(), reqID)

	username, err := parsePostField(post, "User")
	require.NoError(t, err)
	assert.Equal(t, s.userNames.requestor, username)

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

func (s *MattermostSuite) TestApproval() {
	t := s.T()

	reviewer := s.fakeMattermost.StoreUser(User{Email: s.userNames.reviewer1})

	s.startApp()

	req := s.createAccessRequest([]User{reviewer})
	post, err := s.fakeMattermost.CheckNewPost(s.Context())
	require.NoError(t, err, "no new messages posted")
	directChannelID := s.fakeMattermost.GetDirectChannelFor(s.fakeMattermost.GetBotUser(), reviewer).ID
	assert.Equal(t, directChannelID, post.ChannelID)

	err = s.ruler().ApproveAccessRequest(s.Context(), req.GetName(), "okay")
	require.NoError(t, err)

	postUpdate, err := s.fakeMattermost.CheckPostUpdate(s.Context())
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

func (s *MattermostSuite) TestDenial() {
	t := s.T()

	reviewer := s.fakeMattermost.StoreUser(User{Email: s.userNames.reviewer1})

	s.startApp()

	req := s.createAccessRequest([]User{reviewer})
	post, err := s.fakeMattermost.CheckNewPost(s.Context())
	require.NoError(t, err, "no new messages posted")
	directChannelID := s.fakeMattermost.GetDirectChannelFor(s.fakeMattermost.GetBotUser(), reviewer).ID
	assert.Equal(t, directChannelID, post.ChannelID)

	// max size of request was decreased here: https://github.com/gravitational/teleport/pull/13298
	err = s.ruler().DenyAccessRequest(s.Context(), req.GetName(), "not okay "+strings.Repeat("A", 4000))
	require.NoError(t, err)

	postUpdate, err := s.fakeMattermost.CheckPostUpdate(s.Context())
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

func (s *MattermostSuite) TestReviewComments() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	reviewer := s.fakeMattermost.StoreUser(User{Email: s.userNames.reviewer1})
	directChannelID := s.fakeMattermost.GetDirectChannelFor(s.fakeMattermost.GetBotUser(), reviewer).ID

	s.startApp()

	req := s.createAccessRequest([]User{reviewer})
	s.checkPluginData(req.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})

	post, err := s.fakeMattermost.CheckNewPost(s.Context())
	require.NoError(t, err)
	assert.Equal(t, directChannelID, post.ChannelID)

	err = s.reviewer1().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer1,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	comment, err := s.fakeMattermost.CheckNewPost(s.Context())
	require.NoError(t, err)
	assert.Equal(t, post.ChannelID, comment.ChannelID)
	assert.Equal(t, post.ID, comment.RootID)
	assert.Contains(t, comment.Message, s.userNames.reviewer1+" reviewed the request", "comment must contain a review author")
	assert.Contains(t, comment.Message, "Resolution: ✅ APPROVED", "comment must contain a proposed state")
	assert.Contains(t, comment.Message, "Reason: ```\nokay```", "comment must contain a reason")

	err = s.reviewer2().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer2,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	comment, err = s.fakeMattermost.CheckNewPost(s.Context())
	require.NoError(t, err)
	assert.Equal(t, post.ChannelID, comment.ChannelID)
	assert.Equal(t, post.ID, comment.RootID)
	assert.Contains(t, comment.Message, s.userNames.reviewer2+" reviewed the request", "comment must contain a review author")
	assert.Contains(t, comment.Message, "Resolution: ❌ DENIED", "comment must contain a proposed state")
	assert.Contains(t, comment.Message, "Reason: ```\nnot okay```", "comment must contain a reason")
}

func (s *MattermostSuite) TestApprovalByReview() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	reviewer := s.fakeMattermost.StoreUser(User{Email: s.userNames.reviewer1})

	s.startApp()

	req := s.createAccessRequest([]User{reviewer})
	post, err := s.fakeMattermost.CheckNewPost(s.Context())
	require.NoError(t, err, "no new messages posted")
	directChannelID := s.fakeMattermost.GetDirectChannelFor(s.fakeMattermost.GetBotUser(), reviewer).ID
	assert.Equal(t, directChannelID, post.ChannelID)

	err = s.reviewer1().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer1,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	})
	require.NoError(t, err)

	comment, err := s.fakeMattermost.CheckNewPost(s.Context())
	require.NoError(t, err)
	assert.Equal(t, post.ChannelID, comment.ChannelID)
	assert.Equal(t, post.ID, comment.RootID)
	assert.Contains(t, comment.Message, s.userNames.reviewer1+" reviewed the request", "comment must contain a review author")

	err = s.reviewer2().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer2,
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "finally okay",
	})
	require.NoError(t, err)

	comment, err = s.fakeMattermost.CheckNewPost(s.Context())
	require.NoError(t, err)
	assert.Equal(t, post.ChannelID, comment.ChannelID)
	assert.Equal(t, post.ID, comment.RootID)
	assert.Contains(t, comment.Message, s.userNames.reviewer2+" reviewed the request", "comment must contain a review author")

	// When posting a review, the bot also updates the message to add the amount of reviewers.
	// This update is soon superseded by the "access allowed" update
	_, _ = s.fakeMattermost.CheckPostUpdate(s.Context())

	postUpdate, err := s.fakeMattermost.CheckPostUpdate(s.Context())
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

func (s *MattermostSuite) TestDenialByReview() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	reviewer := s.fakeMattermost.StoreUser(User{Email: s.userNames.reviewer1})

	s.startApp()

	req := s.createAccessRequest([]User{reviewer})
	post, err := s.fakeMattermost.CheckNewPost(s.Context())
	require.NoError(t, err, "no new messages posted")
	directChannelID := s.fakeMattermost.GetDirectChannelFor(s.fakeMattermost.GetBotUser(), reviewer).ID
	assert.Equal(t, directChannelID, post.ChannelID)

	err = s.reviewer1().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer1,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "not okay",
	})
	require.NoError(t, err)

	comment, err := s.fakeMattermost.CheckNewPost(s.Context())
	require.NoError(t, err)
	assert.Equal(t, post.ChannelID, comment.ChannelID)
	assert.Equal(t, post.ID, comment.RootID)
	assert.Contains(t, comment.Message, s.userNames.reviewer1+" reviewed the request", "comment must contain a review author")

	err = s.reviewer2().SubmitAccessRequestReview(s.Context(), req.GetName(), types.AccessReview{
		Author:        s.userNames.reviewer2,
		ProposedState: types.RequestState_DENIED,
		Created:       time.Now(),
		Reason:        "finally not okay",
	})
	require.NoError(t, err)

	// When posting a review, the bot also updates the message to add the amount of reviewers.
	// This update is soon superseded by the "access allowed" update
	_, _ = s.fakeMattermost.CheckPostUpdate(s.Context())

	comment, err = s.fakeMattermost.CheckNewPost(s.Context())
	require.NoError(t, err)
	assert.Equal(t, post.ChannelID, comment.ChannelID)
	assert.Equal(t, post.ID, comment.RootID)
	assert.Contains(t, comment.Message, s.userNames.reviewer2+" reviewed the request", "comment must contain a review author")

	postUpdate, err := s.fakeMattermost.CheckPostUpdate(s.Context())
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

func (s *MattermostSuite) TestExpiration() {
	t := s.T()

	reviewer := s.fakeMattermost.StoreUser(User{Email: "user@example.com"})

	s.startApp()

	request := s.createAccessRequest([]User{reviewer})
	post, err := s.fakeMattermost.CheckNewPost(s.Context())
	require.NoError(t, err, "no new messages posted")
	directChannelID := s.fakeMattermost.GetDirectChannelFor(s.fakeMattermost.GetBotUser(), reviewer).ID
	assert.Equal(t, directChannelID, post.ChannelID)

	s.checkPluginData(request.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})

	err = s.ruler().DeleteAccessRequest(s.Context(), request.GetName()) // simulate expiration
	require.NoError(t, err)

	postUpdate, err := s.fakeMattermost.CheckPostUpdate(s.Context())
	require.NoError(t, err, "no new messages updated")
	assert.Equal(t, post.ID, postUpdate.ID)
	assert.Equal(t, post.ChannelID, postUpdate.ChannelID)

	statusLine, err := parsePostField(postUpdate, "Status")
	require.NoError(t, err)
	assert.Equal(t, "⌛ EXPIRED", statusLine)
}

func (s *MattermostSuite) TestRace() {
	t := s.T()

	if !s.teleportFeatures.AdvancedAccessWorkflows {
		t.Skip("Doesn't work in OSS version")
	}

	err := logger.Setup(logger.Config{Severity: "info"}) // Turn off noisy debug logging
	require.NoError(t, err)

	reviewer1 := s.fakeMattermost.StoreUser(User{Email: s.userNames.reviewer1})
	reviewer2 := s.fakeMattermost.StoreUser(User{Email: s.userNames.reviewer2})
	botUser := s.fakeMattermost.GetBotUser()

	s.SetContextTimeout(20 * time.Second)
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

	process := lib.NewProcess(s.Context())
	for i := 0; i < s.raceNumber; i++ {
		process.SpawnCritical(func(ctx context.Context) error {
			req, err := types.NewAccessRequest(uuid.New().String(), s.userNames.requestor, "editor")
			if err != nil {
				return setRaceErr(trace.Wrap(err))
			}
			req.SetSuggestedReviewers([]string{reviewer1.Email, reviewer2.Email})
			if _, err := s.requestor().CreateAccessRequestV2(ctx, req); err != nil {
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

				if err = s.clients[user.Email].SubmitAccessRequestReview(ctx, reqID, types.AccessReview{
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
				return setRaceErr(trace.Wrap(err))
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

func parsePostField(post Post, field string) (string, error) {
	text := post.Message
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

func (s *MattermostSuite) TestRecipientsConfig() {
	t := s.T()

	reviewer1 := s.fakeMattermost.StoreUser(User{
		Email: s.userNames.reviewer1,
	})
	directChannel1 := s.fakeMattermost.GetDirectChannelFor(s.fakeMattermost.GetBotUser(), reviewer1)

	team := s.fakeMattermost.StoreTeam(Team{Name: "team-llama"})
	channel2 := s.fakeMattermost.StoreChannel(Channel{Name: "channel-llama", TeamID: team.ID})

	// Test an email and a team/channel
	s.appConfig.Recipients = common.RawRecipientsMap{
		types.Wildcard: []string{"team-llama/channel-llama", reviewer1.Email},
	}

	s.startApp()

	request := s.createAccessRequest(nil)
	pluginData := s.checkPluginData(request.GetName(), func(data accessrequest.PluginData) bool {
		return len(data.SentMessages) > 0
	})
	assert.Len(t, pluginData.SentMessages, 2)

	var (
		msg      Post
		messages []Post
	)

	messageSet := make(MattermostDataPostSet)

	msg, err := s.fakeMattermost.CheckNewPost(s.Context())
	require.NoError(t, err)
	messageSet.Add(accessrequest.MessageData{ChannelID: msg.ChannelID, MessageID: msg.ID})
	messages = append(messages, msg)

	msg, err = s.fakeMattermost.CheckNewPost(s.Context())
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
