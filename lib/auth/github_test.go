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
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/trace"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
)

type githubContext struct {
	a           *Server
	mockEmitter *eventstest.MockEmitter
	b           backend.Backend
	c           clockwork.FakeClock
}

func setupGithubContext(ctx context.Context, t *testing.T) *githubContext {
	var tt githubContext
	t.Cleanup(func() { tt.Close() })

	tt.c = clockwork.NewFakeClockAt(time.Now())

	var err error
	tt.b, err = memory.New(memory.Config{
		Context: context.Background(),
		Clock:   tt.c,
	})
	require.NoError(t, err)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)

	authConfig := &InitConfig{
		ClusterName:            clusterName,
		Backend:                tt.b,
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
	}
	tt.a, err = NewServer(authConfig)
	require.NoError(t, err)

	tt.mockEmitter = &eventstest.MockEmitter{}
	tt.a.emitter = tt.mockEmitter

	return &tt
}

func (tt *githubContext) Close() error {
	return trace.NewAggregate(
		tt.a.Close(),
		tt.b.Close())
}

func TestPopulateClaims(t *testing.T) {
	claims, err := populateGithubClaims(&testGithubAPIClient{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(claims, &types.GithubClaims{
		Username: "octocat",
		OrganizationToTeams: map[string][]string{
			"org1": {"team1", "team2"},
			"org2": {"team1"},
		},
		Teams: []string{"team1", "team2", "team1"},
	}))

}

func TestCreateGithubUser(t *testing.T) {
	ctx := context.Background()
	tt := setupGithubContext(ctx, t)

	// Dry-run creation of Github user.
	user, err := tt.a.createGithubUser(context.Background(), &createUserParams{
		connectorName: "github",
		username:      "foo@example.com",
		roles:         []string{"admin"},
		sessionTTL:    1 * time.Minute,
	}, true)
	require.NoError(t, err)
	require.Equal(t, user.GetName(), "foo@example.com")

	// Dry-run must not create a user.
	_, err = tt.a.GetUser("foo@example.com", false)
	require.Error(t, err)

	// Create GitHub user with 1 minute expiry.
	_, err = tt.a.createGithubUser(context.Background(), &createUserParams{
		connectorName: "github",
		username:      "foo",
		roles:         []string{"admin"},
		sessionTTL:    1 * time.Minute,
	}, false)
	require.NoError(t, err)

	// Within that 1 minute period the user should still exist.
	_, err = tt.a.GetUser("foo", false)
	require.NoError(t, err)

	// Advance time 2 minutes, the user should be gone.
	tt.c.Advance(2 * time.Minute)
	_, err = tt.a.GetUser("foo", false)
	require.Error(t, err)
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

func TestValidateGithubAuthCallbackEventsEmitted(t *testing.T) {
	ctx := context.Background()
	tt := setupGithubContext(ctx, t)

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
	_, _ = validateGithubAuthCallbackHelper(context.Background(), m, nil, tt.a.emitter)
	require.Equal(t, tt.mockEmitter.LastEvent().GetType(), events.UserLoginEvent)
	require.Equal(t, tt.mockEmitter.LastEvent().GetCode(), events.UserSSOLoginCode)
	require.Equal(t, ssoDiagInfoCalls, 0)
	tt.mockEmitter.Reset()

	// Test failure event.
	m.mockValidateGithubAuthCallback = func(ctx context.Context, diagCtx *ssoDiagContext, q url.Values) (*GithubAuthResponse, error) {
		diagCtx.info.GithubClaims = claims
		return auth, trace.BadParameter("")
	}
	_, _ = validateGithubAuthCallbackHelper(context.Background(), m, nil, tt.a.emitter)
	require.Equal(t, tt.mockEmitter.LastEvent().GetCode(), events.UserSSOLoginFailureCode)
	require.Equal(t, ssoDiagInfoCalls, 0)

	// TestFlow: true
	m.testFlow = true

	// Test success event.
	m.mockValidateGithubAuthCallback = func(ctx context.Context, diagCtx *ssoDiagContext, q url.Values) (*GithubAuthResponse, error) {
		diagCtx.info.GithubClaims = claims
		return auth, nil
	}
	_, _ = validateGithubAuthCallbackHelper(context.Background(), m, nil, tt.a.emitter)
	require.Equal(t, tt.mockEmitter.LastEvent().GetType(), events.UserLoginEvent)
	require.Equal(t, tt.mockEmitter.LastEvent().GetCode(), events.UserSSOTestFlowLoginCode)
	require.Equal(t, ssoDiagInfoCalls, 1)
	tt.mockEmitter.Reset()

	// Test failure event.
	m.mockValidateGithubAuthCallback = func(ctx context.Context, diagCtx *ssoDiagContext, q url.Values) (*GithubAuthResponse, error) {
		diagCtx.info.GithubClaims = claims
		return auth, trace.BadParameter("")
	}
	_, _ = validateGithubAuthCallbackHelper(context.Background(), m, nil, tt.a.emitter)
	require.Equal(t, tt.mockEmitter.LastEvent().GetCode(), events.UserSSOTestFlowLoginFailureCode)
	require.Equal(t, ssoDiagInfoCalls, 2)
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
