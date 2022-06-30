/*
Copyright 2017 Gravitational, Inc.

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

package auth

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"gopkg.in/check.v1"
)

func TestAPI(t *testing.T) { check.TestingT(t) }

type GithubSuite struct {
	a           *Server
	mockEmitter *eventstest.MockEmitter
	b           backend.Backend
	c           clockwork.FakeClock
}

var _ = check.Suite(&GithubSuite{})

func (s *GithubSuite) SetUpSuite(c *check.C) {
	s.c = clockwork.NewFakeClockAt(time.Now())

	var err error
	s.b, err = lite.NewWithConfig(context.Background(), lite.Config{
		Path:             c.MkDir(),
		PollStreamPeriod: 200 * time.Millisecond,
		Clock:            s.c,
	})
	c.Assert(err, check.IsNil)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	c.Assert(err, check.IsNil)

	authConfig := &InitConfig{
		ClusterName:            clusterName,
		Backend:                s.b,
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
	}
	s.a, err = NewServer(authConfig)
	c.Assert(err, check.IsNil)

	s.mockEmitter = &eventstest.MockEmitter{}
	s.a.emitter = s.mockEmitter
}

func (s *GithubSuite) TestPopulateClaims(c *check.C) {
	claims, err := populateGithubClaims(&testGithubAPIClient{})
	c.Assert(err, check.IsNil)
	c.Assert(claims, check.DeepEquals, &types.GithubClaims{
		Username: "octocat",
		OrganizationToTeams: map[string][]string{
			"org1": {"team1", "team2"},
			"org2": {"team1"},
		},
		Teams: []string{"team1", "team2", "team1"},
	})
}

func (s *GithubSuite) TestCreateGithubUser(c *check.C) {
	// Dry-run creation of Github user.
	user, err := s.a.createGithubUser(context.Background(), &createUserParams{
		connectorName: "github",
		username:      "foo@example.com",
		roles:         []string{"admin"},
		sessionTTL:    1 * time.Minute,
	}, true)
	c.Assert(err, check.IsNil)
	c.Assert(user.GetName(), check.Equals, "foo@example.com")

	// Dry-run must not create a user.
	_, err = s.a.GetUser("foo@example.com", false)
	c.Assert(err, check.NotNil)

	// Create GitHub user with 1 minute expiry.
	_, err = s.a.createGithubUser(context.Background(), &createUserParams{
		connectorName: "github",
		username:      "foo",
		roles:         []string{"admin"},
		sessionTTL:    1 * time.Minute,
	}, false)
	c.Assert(err, check.IsNil)

	// Within that 1 minute period the user should still exist.
	_, err = s.a.GetUser("foo", false)
	c.Assert(err, check.IsNil)

	// Advance time 2 minutes, the user should be gone.
	s.c.Advance(2 * time.Minute)
	_, err = s.a.GetUser("foo", false)
	c.Assert(err, check.NotNil)
}

type testGithubAPIClient struct{}

func (c *testGithubAPIClient) getUser() (*userResponse, error) {
	return &userResponse{Login: "octocat"}, nil
}

func (c *testGithubAPIClient) getTeams() ([]teamResponse, error) {
	return []teamResponse{
		{
			Name: "team1",
			Slug: "team1",
			Org:  orgResponse{Login: "org1"},
		},
		{
			Name: "team2",
			Slug: "team2",
			Org:  orgResponse{Login: "org1"},
		},
		{
			Name: "team1",
			Slug: "team1",
			Org:  orgResponse{Login: "org2"},
		},
	}, nil
}

func (s *GithubSuite) TestValidateGithubAuthCallbackEventsEmitted(c *check.C) {
	auth := &GithubAuthResponse{
		Username: "test-name",
	}

	claims := &types.GithubClaims{
		OrganizationToTeams: map[string][]string{
			"test": {},
		},
	}

	ssoDiagInfoCalls := 0

	m := &mockedGithubManager{}
	m.createSSODiagnosticInfo = func(ctx context.Context, authKind string, authRequestID string, info types.SSODiagnosticInfo) error {
		ssoDiagInfoCalls++
		return nil
	}

	// TestFlow: false
	m.testFlow = false

	// Test success event.
	m.mockValidateGithubAuthCallback = func(ctx context.Context, diagCtx *ssoDiagContext, q url.Values) (*GithubAuthResponse, error) {
		diagCtx.info.GithubClaims = claims
		return auth, nil
	}
	_, _ = validateGithubAuthCallbackHelper(context.Background(), m, nil, s.a.emitter)
	c.Assert(s.mockEmitter.LastEvent().GetType(), check.Equals, events.UserLoginEvent)
	c.Assert(s.mockEmitter.LastEvent().GetCode(), check.Equals, events.UserSSOLoginCode)
	c.Assert(ssoDiagInfoCalls, check.Equals, 0)
	s.mockEmitter.Reset()

	// Test failure event.
	m.mockValidateGithubAuthCallback = func(ctx context.Context, diagCtx *ssoDiagContext, q url.Values) (*GithubAuthResponse, error) {
		diagCtx.info.GithubClaims = claims
		return auth, trace.BadParameter("")
	}
	_, _ = validateGithubAuthCallbackHelper(context.Background(), m, nil, s.a.emitter)
	c.Assert(s.mockEmitter.LastEvent().GetCode(), check.Equals, events.UserSSOLoginFailureCode)
	c.Assert(ssoDiagInfoCalls, check.Equals, 0)

	// TestFlow: true
	m.testFlow = true

	// Test success event.
	m.mockValidateGithubAuthCallback = func(ctx context.Context, diagCtx *ssoDiagContext, q url.Values) (*GithubAuthResponse, error) {
		diagCtx.info.GithubClaims = claims
		return auth, nil
	}
	_, _ = validateGithubAuthCallbackHelper(context.Background(), m, nil, s.a.emitter)
	c.Assert(s.mockEmitter.LastEvent().GetType(), check.Equals, events.UserLoginEvent)
	c.Assert(s.mockEmitter.LastEvent().GetCode(), check.Equals, events.UserSSOTestFlowLoginCode)
	c.Assert(ssoDiagInfoCalls, check.Equals, 1)
	s.mockEmitter.Reset()

	// Test failure event.
	m.mockValidateGithubAuthCallback = func(ctx context.Context, diagCtx *ssoDiagContext, q url.Values) (*GithubAuthResponse, error) {
		diagCtx.info.GithubClaims = claims
		return auth, trace.BadParameter("")
	}
	_, _ = validateGithubAuthCallbackHelper(context.Background(), m, nil, s.a.emitter)
	c.Assert(s.mockEmitter.LastEvent().GetCode(), check.Equals, events.UserSSOTestFlowLoginFailureCode)
	c.Assert(ssoDiagInfoCalls, check.Equals, 2)
}

type mockedGithubManager struct {
	mockValidateGithubAuthCallback func(ctx context.Context, diagCtx *ssoDiagContext, q url.Values) (*GithubAuthResponse, error)
	createSSODiagnosticInfo        func(ctx context.Context, authKind string, authRequestID string, info types.SSODiagnosticInfo) error

	testFlow bool
}

func (m *mockedGithubManager) newSSODiagContext(authKind string) *ssoDiagContext {
	if m.createSSODiagnosticInfo == nil {
		panic("mockedGithubManager.createSSODiagnosticInfo is nil, newSSODiagContext cannot succeed.")
	}

	return &ssoDiagContext{
		authKind:                authKind,
		createSSODiagnosticInfo: m.createSSODiagnosticInfo,
		requestID:               uuid.New().String(),
		info:                    types.SSODiagnosticInfo{TestFlow: m.testFlow},
	}
}

func (m *mockedGithubManager) validateGithubAuthCallback(ctx context.Context, diagCtx *ssoDiagContext, q url.Values) (*GithubAuthResponse, error) {
	if m.mockValidateGithubAuthCallback != nil {
		return m.mockValidateGithubAuthCallback(ctx, diagCtx, q)
	}

	return nil, trace.NotImplemented("mockValidateGithubAuthCallback not implemented")
}
