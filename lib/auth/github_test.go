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

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/keystore"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/loginrule"
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

func TestCalculateGithubUserNoTeams(t *testing.T) {
	ctx := context.Background()
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

	_, err = a.calculateGithubUser(ctx, connector, &types.GithubClaims{
		Username: "octocat",
		OrganizationToTeams: map[string][]string{
			"org1": {"team1", "team2"},
			"org2": {"team1"},
		},
		Teams: []string{"team1", "team2", "team1"},
	}, &types.GithubAuthRequest{})
	require.ErrorIs(t, err, ErrGithubNoTeams)
}

// Test that calculateGithubUser calls the login rule evaluator, evaluated
// traits end up in the user params, and traits are evaluated exactly once.
func TestCalculateGithubUserWithLoginRules(t *testing.T) {
	ctx := context.Background()

	// Create a test role so that FetchRoles can succeed.
	roles := map[string]types.Role{
		"access": &types.RoleV5{
			Metadata: types.Metadata{
				Name: "access",
			},
			Spec: types.RoleSpecV5{
				Allow: types.RoleConditions{
					Logins: []string{"{{internal.logins}}"},
				},
			},
		},
	}
	a := &Server{
		Cache: &mockRoleCache{
			roles: roles,
		},
	}

	// Insert a mock login rule evaluator with static outputs, the real login
	// rule evaluator is in the enterprise codebase.
	evaluatedTraits := map[string][]string{
		"logins":                  {"octocat", "octodog"},
		"teams":                   {"access", "team1", "team3"},
		constants.TraitKubeGroups: {"kubers"},
		constants.TraitKubeUsers:  {"k8"},
	}
	mockEvaluator := &mockLoginRuleEvaluator{
		outputTraits: evaluatedTraits,
	}
	a.SetLoginRuleEvaluator(mockEvaluator)

	// Create a basic connector to map to the test role.
	connector, err := types.NewGithubConnector("github", types.GithubConnectorSpecV3{
		TeamsToRoles: []types.TeamRolesMapping{
			{
				Organization: "org1",
				Team:         "team1",
				Roles:        []string{"access"},
			},
		},
	})
	require.NoError(t, err)

	userParams, err := a.calculateGithubUser(ctx, connector, &types.GithubClaims{
		Username: "octocat",
		OrganizationToTeams: map[string][]string{
			"org1": {"team1"},
		},
		Teams: []string{"team1"},
	}, &types.GithubAuthRequest{})
	require.NoError(t, err)

	require.Equal(t, &createUserParams{
		connectorName: "github",
		username:      "octocat",
		kubeGroups:    evaluatedTraits[constants.TraitKubeGroups],
		kubeUsers:     evaluatedTraits[constants.TraitKubeUsers],
		roles:         []string{"access"},
		traits:        evaluatedTraits,
		sessionTTL:    defaults.MaxCertDuration,
	}, userParams, "user params does not match expected")
	require.Equal(t, 1, mockEvaluator.evaluatedCount, "login rules were not evaluated exactly once")
}

type mockRoleCache struct {
	roles map[string]types.Role
	Cache
}

func (m *mockRoleCache) GetRole(_ context.Context, name string) (types.Role, error) {
	return m.roles[name], nil
}

type mockLoginRuleEvaluator struct {
	outputTraits   map[string][]string
	evaluatedCount int
}

func (m *mockLoginRuleEvaluator) Evaluate(context.Context, *loginrule.EvaluationInput) (*loginrule.EvaluationOutput, error) {
	m.evaluatedCount++
	return &loginrule.EvaluationOutput{
		Traits:       m.outputTraits,
		AppliedRules: []string{"mock"},
	}, nil
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

func TestGithubURLFormat(t *testing.T) {
	tts := []struct {
		host   string
		path   string
		expect string
	}{
		{
			host:   "example.com",
			path:   "foo/bar",
			expect: "https://api.example.com/foo/bar",
		},
		{
			host:   "example.com",
			path:   "/foo/bar?spam=eggs",
			expect: "https://api.example.com/foo/bar?spam=eggs",
		},
		{
			host:   "example.com",
			path:   "/foo/bar",
			expect: "https://api.example.com/foo/bar",
		},
	}

	for _, tt := range tts {
		require.Equal(t, tt.expect, formatGithubURL(tt.host, tt.path))
	}
}
