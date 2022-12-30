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
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/keystore"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
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
		KeyStoreConfig: keystore.Config{
			Software: keystore.SoftwareConfig{
				RSAKeyPairSource: authority.New().GenerateKeyPair,
			},
		},
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
	client := &testGithubAPIClient{}
	user, err := client.getUser()
	require.NoError(t, err)
	teams, err := client.getTeams()
	require.NoError(t, err)

	claims, err := populateGithubClaims(user, teams)
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
	user, err := tt.a.createGithubUser(context.Background(), &CreateUserParams{
		ConnectorName: "github",
		Username:      "foo@example.com",
		Roles:         []string{"admin"},
		SessionTTL:    1 * time.Minute,
	}, true)
	require.NoError(t, err)
	require.Equal(t, user.GetName(), "foo@example.com")

	// Dry-run must not create a user.
	_, err = tt.a.GetUser("foo@example.com", false)
	require.Error(t, err)

	// Create GitHub user with 1 minute expiry.
	_, err = tt.a.createGithubUser(context.Background(), &CreateUserParams{
		ConnectorName: "github",
		Username:      "foo",
		Roles:         []string{"admin"},
		SessionTTL:    1 * time.Minute,
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
	createSSODiagnosticInfoStub := func(ctx context.Context, authKind string, authRequestID string, entry types.SSODiagnosticInfo) error {
		ssoDiagInfoCalls++
		return nil
	}

	ssoDiagContextFixture := func(testFlow bool) *SSODiagContext {
		diagCtx := NewSSODiagContext(types.KindGithub, SSODiagServiceFunc(createSSODiagnosticInfoStub))
		diagCtx.RequestID = uuid.New().String()
		diagCtx.Info.TestFlow = testFlow
		return diagCtx
	}
	m := &mockedGithubManager{}

	// Test success event, non-test-flow.
	diagCtx := ssoDiagContextFixture(false /* testFlow */)
	m.mockValidateGithubAuthCallback = func(ctx context.Context, diagCtx *SSODiagContext, q url.Values) (*GithubAuthResponse, error) {
		diagCtx.Info.GithubClaims = claims
		return auth, nil
	}
	_, _ = validateGithubAuthCallbackHelper(context.Background(), m, diagCtx, nil, tt.a.emitter)
	require.Equal(t, tt.mockEmitter.LastEvent().GetType(), events.UserLoginEvent)
	require.Equal(t, tt.mockEmitter.LastEvent().GetCode(), events.UserSSOLoginCode)
	require.Equal(t, ssoDiagInfoCalls, 0)
	tt.mockEmitter.Reset()

	// Test failure event, non-test-flow.
	diagCtx = ssoDiagContextFixture(false /* testFlow */)
	m.mockValidateGithubAuthCallback = func(ctx context.Context, diagCtx *SSODiagContext, q url.Values) (*GithubAuthResponse, error) {
		diagCtx.Info.GithubClaims = claims
		return auth, trace.BadParameter("")
	}
	_, _ = validateGithubAuthCallbackHelper(context.Background(), m, diagCtx, nil, tt.a.emitter)
	require.Equal(t, tt.mockEmitter.LastEvent().GetCode(), events.UserSSOLoginFailureCode)
	require.Equal(t, ssoDiagInfoCalls, 0)

	// Test success event, test-flow.
	diagCtx = ssoDiagContextFixture(true /* testFlow */)
	m.mockValidateGithubAuthCallback = func(ctx context.Context, diagCtx *SSODiagContext, q url.Values) (*GithubAuthResponse, error) {
		diagCtx.Info.GithubClaims = claims
		return auth, nil
	}
	_, _ = validateGithubAuthCallbackHelper(context.Background(), m, diagCtx, nil, tt.a.emitter)
	require.Equal(t, tt.mockEmitter.LastEvent().GetType(), events.UserLoginEvent)
	require.Equal(t, tt.mockEmitter.LastEvent().GetCode(), events.UserSSOTestFlowLoginCode)
	require.Equal(t, ssoDiagInfoCalls, 1)
	tt.mockEmitter.Reset()

	// Test failure event, test-flow.
	diagCtx = ssoDiagContextFixture(true /* testFlow */)
	m.mockValidateGithubAuthCallback = func(ctx context.Context, diagCtx *SSODiagContext, q url.Values) (*GithubAuthResponse, error) {
		diagCtx.Info.GithubClaims = claims
		return auth, trace.BadParameter("")
	}
	_, _ = validateGithubAuthCallbackHelper(context.Background(), m, diagCtx, nil, tt.a.emitter)
	require.Equal(t, tt.mockEmitter.LastEvent().GetCode(), events.UserSSOTestFlowLoginFailureCode)
	require.Equal(t, ssoDiagInfoCalls, 2)
}

type mockedGithubManager struct {
	mockValidateGithubAuthCallback func(ctx context.Context, diagCtx *SSODiagContext, q url.Values) (*GithubAuthResponse, error)
}

func (m *mockedGithubManager) validateGithubAuthCallback(ctx context.Context, diagCtx *SSODiagContext, q url.Values) (*GithubAuthResponse, error) {
	if m.mockValidateGithubAuthCallback != nil {
		return m.mockValidateGithubAuthCallback(ctx, diagCtx, q)
	}

	return nil, trace.NotImplemented("mockValidateGithubAuthCallback not implemented")
}

func TestCalculateGithubUserNoTeams(t *testing.T) {
	a := &Server{}
	connector, err := types.NewGithubConnector("github", types.GithubConnectorSpecV3{
		TeamsToRoles: []types.TeamRolesMapping{
			{
				Organization: "org1",
				Team:         "teamx",
				Roles:        []string{"role"},
			},
		},
	})
	require.NoError(t, err)

	_, err = a.calculateGithubUser(connector, &types.GithubClaims{
		Username: "octocat",
		OrganizationToTeams: map[string][]string{
			"org1": {"team1", "team2"},
			"org2": {"team1"},
		},
		Teams: []string{"team1", "team2", "team1"},
	}, &types.GithubAuthRequest{})
	require.ErrorIs(t, err, ErrGithubNoTeams)
}

type mockHTTPRequester struct {
	succeed    bool
	statusCode int
}

func (m mockHTTPRequester) Do(req *http.Request) (*http.Response, error) {
	if !m.succeed {
		return nil, &url.Error{
			URL: req.URL.String(),
			Err: &net.DNSError{
				IsTimeout: true,
			},
		}
	}

	resp := new(http.Response)
	resp.Body = io.NopCloser(bytes.NewReader([]byte{}))
	resp.StatusCode = m.statusCode

	return resp, nil
}

func TestCheckGithubOrgSSOSupport(t *testing.T) {
	noSSOOrg, err := types.NewGithubConnector("github-no-sso", types.GithubConnectorSpecV3{
		TeamsToRoles: []types.TeamRolesMapping{
			{
				Organization: "no-sso-org",
				Team:         "team",
				Roles:        []string{"role"},
			},
		},
	})
	require.NoError(t, err)
	ssoOrg, err := types.NewGithubConnector("github-sso", types.GithubConnectorSpecV3{
		TeamsToRoles: []types.TeamRolesMapping{
			{
				Organization: "sso-org",
				Team:         "team",
				Roles:        []string{"role"},
			},
		},
	})
	require.NoError(t, err)

	tests := []struct {
		testName             string
		connector            types.GithubConnector
		isEnterprise         bool
		requestShouldSucceed bool
		httpStatusCode       int
		reuseCache           bool
		errFunc              func(error) bool
	}{
		{
			testName:             "OSS HTTP connection failure",
			connector:            ssoOrg,
			isEnterprise:         false,
			requestShouldSucceed: false,
			reuseCache:           false,
			errFunc:              trace.IsConnectionProblem,
		},
		{
			testName:             "Enterprise skips HTTP check",
			connector:            ssoOrg,
			isEnterprise:         true,
			requestShouldSucceed: false,
			reuseCache:           false,
			errFunc:              nil,
		},
		{
			testName:             "OSS has SSO",
			connector:            ssoOrg,
			isEnterprise:         false,
			requestShouldSucceed: true,
			httpStatusCode:       http.StatusOK,
			reuseCache:           false,
			errFunc:              trace.IsAccessDenied,
		},
		{
			testName:             "OSS has SSO with cache",
			connector:            ssoOrg,
			isEnterprise:         false,
			requestShouldSucceed: false,
			reuseCache:           true,
			errFunc:              trace.IsAccessDenied,
		},
		{
			testName:             "OSS doesn't have SSO",
			connector:            noSSOOrg,
			isEnterprise:         false,
			requestShouldSucceed: true,
			httpStatusCode:       404,
			reuseCache:           true,
			errFunc:              nil,
		},
		{
			testName:             "OSS doesn't have SSO with cache",
			connector:            noSSOOrg,
			isEnterprise:         false,
			requestShouldSucceed: false,
			reuseCache:           true,
			errFunc:              nil,
		},
	}

	var orgCache *utils.FnCache
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			client := mockHTTPRequester{
				succeed:    tt.requestShouldSucceed,
				statusCode: tt.httpStatusCode,
			}

			if tt.isEnterprise {
				modules.SetTestModules(t, &modules.TestModules{
					TestBuildType: modules.BuildEnterprise,
				})
			}

			if !tt.reuseCache {
				orgCache, err = utils.NewFnCache(utils.FnCacheConfig{
					TTL: time.Minute,
				})
				require.NoError(t, err)
			}

			err := checkGithubOrgSSOSupport(ctx, tt.connector, nil, orgCache, client)
			if tt.errFunc == nil {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.True(t, tt.errFunc(err))
			}
		})
	}
}
