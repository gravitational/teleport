package auth

import (
	"context"
	"crypto"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coreos/go-oidc/jose"
	"github.com/coreos/go-oidc/oauth2"
	"github.com/coreos/go-oidc/oidc"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	directory "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/cloudidentity/v1"
	"google.golang.org/api/option"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/e/lib/loginrule"
	"github.com/gravitational/teleport/e/lib/loginrule/storage"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	testserver "github.com/gravitational/teleport/tool/teleport/testenv"
)

type OIDCSuite struct {
	a       *auth.Server
	b       backend.Backend
	c       clockwork.FakeClock
	oas     *OIDCAuthService
	emitter *eventstest.MockRecorderEmitter
}

func setUpSuite(t *testing.T) *OIDCSuite {
	s := OIDCSuite{
		emitter: &eventstest.MockRecorderEmitter{},
	}

	ctx := context.Background()
	s.c = clockwork.NewFakeClockAt(time.Now())

	var err error
	s.b, err = memory.New(memory.Config{
		Context: ctx,
		Clock:   s.c,
	})
	require.NoError(t, err)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)

	authConfig := &auth.InitConfig{
		VersionStorage:         auth.NewFakeTeleportVersion(),
		ClusterName:            clusterName,
		Backend:                s.b,
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
	}
	s.a, err = auth.NewServer(authConfig)
	require.NoError(t, err)

	s.oas, err = NewOIDCAuthService(&OIDCAuthServiceConfig{Auth: s.a, License: ValidLicense{}, Emitter: s.emitter})
	require.NoError(t, err)
	s.a.SetOIDCService(s.oas)

	return &s
}

// createInsecureOIDCClient creates an insecure client for testing.
func createInsecureOIDCClient(t *testing.T, connector types.OIDCConnector) *oidc.Client {
	conf := oidcConfig(connector, "")
	conf.HTTPClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	client, err := oidc.NewClient(conf)
	require.NoError(t, err)
	client.SyncProviderConfig(context.Background(), connector.GetIssuerURL())
	return client
}

func TestCreateOIDCUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	s := setUpSuite(t)

	// Dry-run creation of OIDC user.
	user, err := s.oas.createOIDCUser(ctx, &auth.CreateUserParams{
		ConnectorName: "oidcService",
		Username:      "foo@example.com",
		Roles:         []string{"admin"},
		SessionTTL:    1 * time.Minute,
	}, true)
	require.NoError(t, err)
	require.Equal(t, "foo@example.com", user.GetName())

	// Dry-run must not create a user.
	_, err = s.a.GetUser(ctx, "foo@example.com", false)
	require.Error(t, err)

	// Create OIDC user with 1 minute expiry.
	_, err = s.oas.createOIDCUser(ctx, &auth.CreateUserParams{
		ConnectorName: "oidcService",
		Username:      "foo@example.com",
		Roles:         []string{"admin"},
		SessionTTL:    1 * time.Minute,
	}, false)
	require.NoError(t, err)

	// Within that 1 minute period the user should still exist.
	user, err = s.a.GetUser(ctx, "foo@example.com", false)
	require.NoError(t, err)

	// Create the same user again and validate that the user was
	// successfully updated
	user2, err := s.oas.createOIDCUser(ctx, &auth.CreateUserParams{
		ConnectorName: "oidcService",
		Username:      "foo@example.com",
		Roles:         []string{"admin"},
		SessionTTL:    1 * time.Minute,
	}, false)
	require.NoError(t, err)
	require.NotEqual(t, user.GetRevision(), user2.GetRevision())
	require.Equal(t, user.GetName(), user2.GetName())

	// Advance time 2 minutes, the user should be gone.
	s.c.Advance(2 * time.Minute)
	_, err = s.a.GetUser(ctx, "foo@example.com", false)
	require.Error(t, err)
}

// TestUserInfoBlockHTTP ensures that an insecure userinfo endpoint returns
// trace.NotFound similar to an invalid userinfo endpoint. For these users,
// all claim information is already within the token and additional claim
// information does not need to be fetched.
func TestUserInfoBlockHTTP(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := setUpSuite(t)

	// Create configurable IdP to use in tests.
	idp := NewFakeOIDCIdP(t, false)

	// Create OIDC connector and client.
	connector, err := types.NewOIDCConnector("test-connector", types.OIDCConnectorSpecV3{
		IssuerURL:     idp.S.URL,
		ClientID:      "00000000000000000000000000000000",
		ClientSecret:  "0000000000000000000000000000000000000000000000000000000000000000",
		ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
		RedirectURLs:  []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
	})
	require.NoError(t, err)

	oidcClient, err := s.oas.getCachedOIDCClient(ctx, connector, "", false)
	require.NoError(t, err)

	// Verify HTTP endpoints return trace.NotFound.
	_, err = claimsFromUserInfo(oidcClient.client, idp.S.URL, "")
	fixtures.AssertNotFound(t, err)
}

// TestUserInfoBadStatus asserts that a 4xx response from userinfo results
// in AccessDenied.
func TestUserInfoBadStatus(t *testing.T) {
	t.Parallel()

	// Create configurable IdP to use in tests.
	idp := NewFakeOIDCIdP(t, true)

	// Create OIDC connector and client.
	connector, err := types.NewOIDCConnector("test-connector", types.OIDCConnectorSpecV3{
		IssuerURL:     idp.S.URL,
		ClientID:      "00000000000000000000000000000000",
		ClientSecret:  "0000000000000000000000000000000000000000000000000000000000000000",
		ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
		RedirectURLs:  []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
	})
	require.NoError(t, err)
	oidcClient := createInsecureOIDCClient(t, connector)

	// Verify HTTP endpoints return trace.AccessDenied.
	_, err = claimsFromUserInfo(oidcClient, idp.S.URL, "")
	fixtures.AssertAccessDenied(t, err)
}

func TestSSODiagnostic(t *testing.T) {
	t.Parallel()

	var loginHookCounter atomic.Int32
	var loginHook auth.LoginHook = func(context.Context, types.User) error {
		loginHookCounter.Add(1)
		return nil
	}

	tests := []struct {
		name            string
		claimsToRoles   []types.ClaimMapping
		claims          map[string]any
		traitsMap       map[string][]string
		expectRoles     []string
		expectTraits    map[string][]string
		wantValidateErr error
		loginHooks      []auth.LoginHook
	}{
		{
			name: "success",
			claimsToRoles: []types.ClaimMapping{
				{
					Claim: "groups",
					Value: "idp-admin",
					Roles: []string{"access"},
				},
			},
			claims: map[string]any{
				"email_verified": true,
				"groups":         []string{"everyone", "idp-admin", "idp-dev"},
				"email":          "superuser@example.com",
				"sub":            "00001234abcd",
				"exp":            1652091713.0,
			},
			expectRoles: []string{"access"},
			expectTraits: map[string][]string{
				"email":  {"superuser@example.com"},
				"groups": {"everyone", "idp-admin", "idp-dev"},
				"sub":    {"00001234abcd"},
			},
			loginHooks: []auth.LoginHook{
				loginHook,
				loginHook,
			},
		},
		{
			name: "fail to map claims to roles",
			claimsToRoles: []types.ClaimMapping{
				{
					Claim: "groups",
					Value: "nonexistant",
					Roles: []string{"access"},
				},
			},
			claims: map[string]any{
				"email_verified": true,
				"groups":         []string{"everyone", "idp-admin", "idp-dev"},
				"email":          "superuser@example.com",
				"sub":            "00001234abcd",
				"exp":            1652091713.0,
			},
			wantValidateErr: ErrOIDCNoRoles,
		},
		{
			// Test that login rules can influence mapped roles.
			name: "login rules",
			claimsToRoles: []types.ClaimMapping{
				{
					Claim: "groups",
					Value: "rule-access",
					Roles: []string{"access"},
				},
			},
			claims: map[string]any{
				"groups": []string{"everyone", "idp-admin", "idp-dev"},
				"email":  "superuser@example.com",
				"sub":    "00001234abcd",
			},
			traitsMap: map[string][]string{
				"email": {"external.email"},
				"groups": {
					`ifelse(external.groups.contains("idp-admin"),
						external.groups.add("rule-access"),
						external.groups)`,
				},
			},
			expectRoles: []string{"access"},
			expectTraits: map[string][]string{
				"email":  {"superuser@example.com"},
				"groups": {"everyone", "idp-admin", "idp-dev", "rule-access"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s := setUpSuite(t)

			loginHookCounter.Store(0)
			for _, hook := range tc.loginHooks {
				s.a.RegisterLoginHook(hook)
			}

			var expectLoginRules []string
			if len(tc.traitsMap) > 0 {
				installLoginRule(ctx, t, s.a, s.b, tc.traitsMap)
				expectLoginRules = append(expectLoginRules, "testrule")
			}

			// Create configurable IdP to use in tests.
			idp := NewFakeOIDCIdP(t, false /* tls */)

			// create role referenced in request.
			_, err := auth.CreateRole(ctx, s.a, "access", types.RoleSpecV6{
				Allow: types.RoleConditions{
					Logins: []string{"dummy"},
				},
			})
			require.NoError(t, err)

			// connector spec
			spec := types.OIDCConnectorSpecV3{
				IssuerURL:     idp.S.URL,
				ClientID:      "00000000000000000000000000000000",
				ClientSecret:  "0000000000000000000000000000000000000000000000000000000000000000",
				Display:       "Test",
				Scope:         []string{"groups"},
				ClaimsToRoles: tc.claimsToRoles,
				RedirectURLs:  []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
			}

			addr := utils.MustParseAddr("1.1.1.1:42")
			oidcRequest := types.OIDCAuthRequest{
				ConnectorID:   "-sso-test-okta",
				Type:          constants.OIDC,
				CertTTL:       defaults.OIDCAuthRequestTTL,
				SSOTestFlow:   true,
				ConnectorSpec: &spec,
				ClientLoginIP: addr.String(),
			}

			request, err := s.a.CreateOIDCAuthRequest(ctx, oidcRequest)
			require.NoError(t, err)
			require.NotNil(t, request)
			require.NotEmpty(t, request.RedirectURL)

			values := url.Values{
				"code":  []string{"XXX-code"},
				"state": []string{request.StateToken},
			}

			// override getClaimsFun.
			s.oas.getClaimsFun = func(closeCtx context.Context, oidcClient *oidc.Client, connector types.OIDCConnector, code string) (jose.Claims, error) {
				return tc.claims, nil
			}

			s.emitter.Reset()
			resp, err := s.oas.ValidateOIDCAuthCallback(ctx, values)
			if tc.wantValidateErr != nil {
				require.ErrorIs(t, err, tc.wantValidateErr)
				return
			}

			require.Len(t, tc.loginHooks, int(loginHookCounter.Load()))

			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Equal(t, &authclient.OIDCAuthResponse{
				Username: "superuser@example.com",
				Identity: types.ExternalIdentity{
					ConnectorID: "-sso-test-okta",
					Username:    "superuser@example.com",
				},
				Req: OIDCAuthRequestFromProto(request),
			}, resp)
			require.NotNil(t, s.emitter.LastEvent())
			require.Equal(t, events.UserLoginEvent, s.emitter.LastEvent().GetType())
			require.IsType(t, &apievents.UserLogin{}, s.emitter.LastEvent())
			loginEvt := s.emitter.LastEvent().(*apievents.UserLogin)
			require.Equal(t, addr.String(), loginEvt.ConnectionMetadata.RemoteAddr)

			diagCtx := auth.SSODiagContext{}

			resp, loginIP, err := s.oas.validateOIDCAuthCallback(ctx, &diagCtx, values)
			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Equal(t, &authclient.OIDCAuthResponse{
				Username: "superuser@example.com",
				Identity: types.ExternalIdentity{
					ConnectorID: "-sso-test-okta",
					Username:    "superuser@example.com",
				},
				Req: OIDCAuthRequestFromProto(request),
			}, resp)
			diff := cmp.Diff(types.SSODiagnosticInfo{
				TestFlow: true,
				Success:  true,
				CreateUserParams: &types.CreateUserParams{
					ConnectorName: "-sso-test-okta",
					Username:      "superuser@example.com",
					Logins:        nil,
					KubeGroups:    nil,
					KubeUsers:     nil,
					Roles:         tc.expectRoles,
					Traits:        tc.expectTraits,
					SessionTTL:    600000000000,
				},
				OIDCClaimsToRoles:         tc.claimsToRoles,
				OIDCClaimsToRolesWarnings: nil,
				OIDCClaims:                tc.claims,
				OIDCIdentity: &types.OIDCIdentity{
					ID:        "00001234abcd",
					Name:      "",
					Email:     "superuser@example.com",
					ExpiresAt: diagCtx.Info.OIDCIdentity.ExpiresAt,
				},
				OIDCTraitsFromClaims: tc.expectTraits,
				OIDCConnectorTraitMapping: []types.TraitMapping{
					{
						Trait: tc.claimsToRoles[0].Claim,
						Value: tc.claimsToRoles[0].Value,
						Roles: tc.claimsToRoles[0].Roles,
					},
				},
				AppliedLoginRules: expectLoginRules,
			}, diagCtx.Info, cmpopts.SortSlices(func(a, b string) bool { return a < b }))
			require.Empty(t, diff, "diagnostic info does not match expected")
			require.Equal(t, addr.String(), loginIP)

			require.Equal(t, len(tc.loginHooks)*2, int(loginHookCounter.Load()))
		})
	}
}

func installLoginRule(ctx context.Context, t *testing.T, a *auth.Server, b backend.Backend, traitsMap map[string][]string) {
	// Install login rules plugin.
	ruleStorage := storage.New(b)
	evaluator := loginrule.NewEvaluator(ruleStorage)
	a.SetLoginRuleEvaluator(evaluator)

	if len(traitsMap) == 0 {
		return
	}

	// Create login rule and upsert to backend.
	rule := &loginrulepb.LoginRule{
		Metadata: &types.Metadata{
			Name: "testrule",
		},
		TraitsMap: make(map[string]*wrappers.StringValues),
	}
	for trait, values := range traitsMap {
		rule.TraitsMap[trait] = &wrappers.StringValues{
			Values: values,
		}
	}
	_, err := ruleStorage.CreateLoginRule(ctx, rule)
	require.NoError(t, err)
}

// TestPingProvider confirms that the client_secret_post auth
// method was set for a oauthclient.
func TestPingProvider(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := setUpSuite(t)

	// Create configurable IdP to use in tests.
	idp := NewFakeOIDCIdP(t, false /* tls */)

	// Create and upsert oidc connector into identity
	connector, err := types.NewOIDCConnector("test-connector", types.OIDCConnectorSpecV3{
		IssuerURL:     idp.S.URL,
		ClientID:      "00000000000000000000000000000000",
		ClientSecret:  "0000000000000000000000000000000000000000000000000000000000000000",
		Provider:      teleport.Ping,
		ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
		RedirectURLs:  []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
	})
	require.NoError(t, err)
	_, err = s.a.CreateOIDCConnector(ctx, connector)
	require.NoError(t, err)

	for _, req := range []types.OIDCAuthRequest{
		{
			ConnectorID: "test-connector",
		}, {
			SSOTestFlow: true,
			ConnectorID: "test-connector",
			ConnectorSpec: &types.OIDCConnectorSpecV3{
				IssuerURL:     idp.S.URL,
				ClientID:      "00000000000000000000000000000000",
				ClientSecret:  "0000000000000000000000000000000000000000000000000000000000000000",
				Provider:      teleport.Ping,
				ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
				RedirectURLs:  []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
			},
		},
	} {
		t.Run(fmt.Sprintf("Test SSOFlow: %v", req.SSOTestFlow), func(t *testing.T) {
			oidcConnector, oidcClient, err := s.oas.getOIDCConnectorAndClient(ctx, req, false)
			require.NoError(t, err)

			oac, err := getOAuthClient(oidcClient, oidcConnector)
			require.NoError(t, err)

			// authMethod should be client secret post now
			require.Equal(t, oauth2.AuthMethodClientSecretPost, oac.GetAuthMethod())
		})
	}
}

func TestOIDCClientProviderSync(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// Create configurable IdP to use in tests.
	idp := NewFakeOIDCIdP(t, false /* tls */)

	// Create OIDC connector and client.
	connector, err := types.NewOIDCConnector("test-connector", types.OIDCConnectorSpecV3{
		IssuerURL:     idp.S.URL,
		ClientID:      "00000000000000000000000000000000",
		ClientSecret:  "0000000000000000000000000000000000000000000000000000000000000000",
		Provider:      teleport.Ping,
		ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
		RedirectURLs:  []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
	})
	require.NoError(t, err)

	client, err := newOIDCClient(ctx, connector, "proxy.example.com")
	require.NoError(t, err)

	// first sync should complete successfully
	require.NoError(t, client.waitFirstSync(100*time.Millisecond))
	require.NoError(t, client.syncCtx.Err())

	// Create OIDC client with a canceled ctx
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()

	client, err = newOIDCClient(canceledCtx, connector, "proxy.example.com")
	require.NoError(t, err)

	// provider sync goroutine should end and first sync should fail
	require.ErrorIs(t, client.syncCtx.Err(), context.Canceled)
	err = client.waitFirstSync(100 * time.Millisecond)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)

	// Create OIDC connector and client without an issuer URL for provider syncing
	connectorNoIssuer, err := types.NewOIDCConnector("test-connector", types.OIDCConnectorSpecV3{
		ClientID:      "00000000000000000000000000000000",
		ClientSecret:  "0000000000000000000000000000000000000000000000000000000000000000",
		Provider:      teleport.Ping,
		ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
		RedirectURLs:  []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
	})
	require.NoError(t, err)

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	client, err = newOIDCClient(timeoutCtx, connectorNoIssuer, "proxy.example.com")
	require.NoError(t, err)

	// first sync should fail after the given timeout and cancel the sync goroutine.
	err = client.waitFirstSync(100 * time.Millisecond)
	require.Error(t, err)
	require.True(t, trace.IsConnectionProblem(err))
	require.ErrorIs(t, client.syncCtx.Err(), context.Canceled)
}

func TestOIDCClientCache(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := setUpSuite(t)

	// Create configurable IdP to use in tests.
	idp := NewFakeOIDCIdP(t, false /* tls */)
	connectorSpec := types.OIDCConnectorSpecV3{
		IssuerURL:     idp.S.URL,
		ClientID:      "00000000000000000000000000000000",
		ClientSecret:  "0000000000000000000000000000000000000000000000000000000000000000",
		Provider:      teleport.Ping,
		ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
		RedirectURLs:  []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
	}
	connector, err := types.NewOIDCConnector("test-connector", connectorSpec)
	require.NoError(t, err)

	// Create and cache a new oidc client
	client, err := s.oas.getCachedOIDCClient(ctx, connector, "proxy.example.com", false)
	require.NoError(t, err)

	// The next call should return the same client (compare memory address)
	cachedClient, err := s.oas.getCachedOIDCClient(ctx, connector, "proxy.example.com", false)
	require.NoError(t, err)
	require.Same(t, client, cachedClient)

	// Canceling provider sync on a cached client should cause it to be replaced
	client.syncCancel()
	cachedClient, err = s.oas.getCachedOIDCClient(ctx, connector, "proxy.example.com", false)
	require.NoError(t, err)
	require.NotSame(t, client, cachedClient)

	// Certain changes to the connector should cause the cached client to be refreshed
	originalClient := cachedClient
	for _, tc := range []struct {
		desc            string
		mutateConnector func(types.OIDCConnector)
		clientAssertion require.ComparisonAssertionFunc
	}{
		{
			desc: "IssuerURL",
			mutateConnector: func(conn types.OIDCConnector) {
				conn.SetIssuerURL(NewFakeOIDCIdP(t, false /* tls */).S.URL)
			},
			clientAssertion: require.NotSame,
		},
		{
			desc: "ClientID",
			mutateConnector: func(conn types.OIDCConnector) {
				conn.SetClientID("11111111111111111111111111111111")
			},
			clientAssertion: require.NotSame,
		},
		{
			desc: "ClientSecret",
			mutateConnector: func(conn types.OIDCConnector) {
				conn.SetClientSecret("1111111111111111111111111111111111111111111111111111111111111111")
			},
			clientAssertion: require.NotSame,
		},
		{
			desc: "RedirectURLs",
			mutateConnector: func(conn types.OIDCConnector) {
				conn.SetRedirectURLs([]string{"https://other.example.com/v1/webapi/oidc/callback"})
			},
			clientAssertion: require.NotSame,
		},
		{
			desc: "Scope",
			mutateConnector: func(conn types.OIDCConnector) {
				conn.SetScope([]string{"groups"})
			},
			clientAssertion: require.NotSame,
		},
		{
			desc: "Prompt - no refresh",
			mutateConnector: func(conn types.OIDCConnector) {
				conn.SetPrompt("none")
			},
			clientAssertion: require.Same,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			newConnector, err := types.NewOIDCConnector("test-connector", connectorSpec)
			require.NoError(t, err)
			tc.mutateConnector(newConnector)

			client, err = s.oas.getCachedOIDCClient(ctx, newConnector, "proxy.example.com", false)
			require.NoError(t, err)
			tc.clientAssertion(t, client, originalClient)

			// reset cached client to the original client for remaining tests
			originalClient, err = s.oas.getCachedOIDCClient(ctx, connector, "proxy.example.com", false)
			require.NoError(t, err)
		})
	}
}

func TestOIDCGoogle(t *testing.T) {
	t.Parallel()

	directGroups := map[string][]string{
		"alice@foo.example":  {"group1@foo.example", "group2@sub.foo.example", "group3@bar.example"},
		"bob@foo.example":    {"group1@foo.example"},
		"carlos@bar.example": {"group1@foo.example", "group2@sub.foo.example", "group3@bar.example"},
	}

	// group2@sub.foo.example is in group3@bar.example and group3@bar.example is in group4@bar.example
	strictDirectGroups := map[string][]string{
		"alice@foo.example":  {"group1@foo.example", "group2@sub.foo.example"},
		"bob@foo.example":    {"group1@foo.example"},
		"carlos@bar.example": {"group1@foo.example", "group2@sub.foo.example"},
	}
	directIndirectGroups := map[string][]string{
		"alice@foo.example":  {"group3@bar.example"},
		"bob@foo.example":    {},
		"carlos@bar.example": {"group3@bar.example"},
	}
	indirectGroups := map[string][]string{
		"alice@foo.example":  {"group4@bar.example"},
		"bob@foo.example":    {},
		"carlos@bar.example": {"group4@bar.example"},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/admin/directory/v1/groups", func(rw http.ResponseWriter, r *http.Request) {
		require.Equal(t, "GET", r.Method)

		email := r.URL.Query().Get("userKey")
		require.NotEmpty(t, email)
		require.Contains(t, directGroups, email)

		domain := r.URL.Query().Get("domain")

		resp := &directory.Groups{}
		for _, groupEmail := range directGroups[email] {
			if domain == "" || strings.HasSuffix(groupEmail, "@"+domain) {
				resp.Groups = append(resp.Groups, &directory.Group{Email: groupEmail})
			}
		}

		require.NoError(t, json.NewEncoder(rw).Encode(resp))
	})
	mux.HandleFunc("/v1/groups/-/memberships:searchTransitiveGroups", func(rw http.ResponseWriter, r *http.Request) {
		require.Equal(t, "GET", r.Method)
		q := r.URL.Query().Get("query")

		// hacky solution but the query parameter of searchTransitiveGroups is also pretty hacky
		prefix := "member_key_id == '"
		suffix := "' && 'cloudidentity.googleapis.com/groups.discussion_forum' in labels"
		require.True(t, strings.HasPrefix(q, prefix))
		require.True(t, strings.HasSuffix(q, suffix))
		email := strings.TrimSuffix(strings.TrimPrefix(q, prefix), suffix)
		require.NotEmpty(t, email)
		require.Contains(t, directGroups, email)

		resp := &cloudidentity.SearchTransitiveGroupsResponse{}

		for relationType, groupEmails := range map[string][]string{
			"DIRECT":              strictDirectGroups[email],
			"DIRECT_AND_INDIRECT": directIndirectGroups[email],
			"INDIRECT":            indirectGroups[email],
		} {
			for _, groupEmail := range groupEmails {
				resp.Memberships = append(resp.Memberships, &cloudidentity.GroupRelation{
					GroupKey: &cloudidentity.EntityKey{
						Id: groupEmail,
					},
					Labels: map[string]string{
						"cloudidentity.googleapis.com/groups.discussion_forum": "",
					},
					RelationType: relationType,
				})
			}
		}

		require.NoError(t, json.NewEncoder(rw).Encode(resp))
	})

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	testOptions := []option.ClientOption{option.WithEndpoint(ts.URL), option.WithoutAuthentication()}

	ctx := context.Background()

	for _, testCase := range []struct {
		email, domain                string
		transitive, direct, filtered []string
	}{
		{
			"alice@foo.example", "foo.example",
			[]string{"group1@foo.example", "group2@sub.foo.example", "group3@bar.example", "group4@bar.example"},
			[]string{"group1@foo.example", "group2@sub.foo.example", "group3@bar.example"},
			[]string{"group1@foo.example"},
		},
		{
			"bob@foo.example", "foo.example",
			[]string{"group1@foo.example"},
			[]string{"group1@foo.example"},
			[]string{"group1@foo.example"},
		},
		{
			"carlos@bar.example", "bar.example",
			[]string{"group1@foo.example", "group2@sub.foo.example", "group3@bar.example", "group4@bar.example"},
			[]string{"group1@foo.example", "group2@sub.foo.example", "group3@bar.example"},
			[]string{"group3@bar.example"},
		},
	} {
		// transitive groups
		groups, err := groupsFromGoogleCloudIdentity(ctx, testCase.email, testOptions...)
		require.NoError(t, err)
		require.ElementsMatch(t, testCase.transitive, groups)

		// direct groups, unfiltered
		groups, err = groupsFromGoogleDirectory(ctx, testCase.email, "", testOptions...)
		require.NoError(t, err)
		require.ElementsMatch(t, testCase.direct, groups)

		// direct groups, filtered by domain
		groups, err = groupsFromGoogleDirectory(ctx, testCase.email, testCase.domain, testOptions...)
		require.NoError(t, err)
		require.ElementsMatch(t, testCase.filtered, groups)
	}
}

func TestEmailVerifiedClaim(t *testing.T) {
	tests := []struct {
		claims        map[string]interface{}
		expectedError string
	}{
		{
			claims: map[string]interface{}{
				"email_verified": "true",
			},
			expectedError: "",
		},
		{
			claims: map[string]interface{}{
				"email_verified": "false",
			},
			expectedError: "email not verified by OIDC provider",
		},
		{
			claims: map[string]interface{}{
				"email_verified": false,
			},
			expectedError: "email not verified by OIDC provider",
		},
		{
			claims: map[string]interface{}{
				"email_verified": true,
			},
			expectedError: "",
		},
		{
			claims: map[string]interface{}{
				"email_verified": "random_value",
			},
			expectedError: "unable to parse oidc claim: \"email_verified\", must be either 'true' or 'false', got 'random_value'",
		},
	}

	for _, test := range tests {
		err := checkEmailVerifiedClaim(test.claims)
		if test.expectedError == "" {
			require.NoError(t, err)
		} else {
			require.ErrorContains(t, err, test.expectedError)
		}
	}
}

// TestUsernameClaim ensures that the `username_claim` field in an OIDC config is handled correctly.
func TestUsernameClaim(t *testing.T) {
	ctx := context.Background()
	s := setUpSuite(t)
	idp := NewFakeOIDCIdP(t, false /* tls */)

	diagCtx := auth.SSODiagContext{}

	// Create role that will be mapped to the user.
	_, err := auth.CreateRole(ctx, s.a, "access", types.RoleSpecV6{
		Allow: types.RoleConditions{},
	})
	require.NoError(t, err)
	require.NoError(t, err)

	// Create claims with "preferred_username" field.
	claims := map[string]interface{}{
		"email_verified":     true,
		"groups":             []string{"everyone"},
		"email":              "test-user@example.com",
		"sub":                "00001234abcd",
		"exp":                1652091713.0,
		"preferred_username": "Teleport_TestUser",
	}

	// Create identity from the claims.
	ident, err := oidc.IdentityFromClaims(claims)
	require.NoError(t, err)

	tests := []struct {
		desc             string
		spec             types.OIDCConnectorSpecV3
		expectedUsername string
		expectedError    string
	}{
		{
			desc: "username_claim specified with correct claim (login hooks called)",
			spec: types.OIDCConnectorSpecV3{
				IssuerURL:     idp.S.URL,
				ClientID:      "000",
				ClientSecret:  "0000",
				ClaimsToRoles: []types.ClaimMapping{{Claim: "groups", Value: "everyone", Roles: []string{"access"}}},
				RedirectURLs:  []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
				UsernameClaim: "preferred_username",
			},
			expectedUsername: "Teleport_TestUser",
		},
		{
			desc: "username_claim specified with incorrect claim",
			spec: types.OIDCConnectorSpecV3{
				IssuerURL:     idp.S.URL,
				ClientID:      "000",
				ClientSecret:  "0000",
				ClaimsToRoles: []types.ClaimMapping{{Claim: "groups", Value: "everyone", Roles: []string{"access"}}},
				RedirectURLs:  []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
				UsernameClaim: "prefred_usrnam",
			},
			expectedError: "The configured username_claim of \"prefred_usrnam\" was not received from the IdP. Please update the username_claim in connector \"okta-oidc\".",
		},
		{
			desc: "no username_claim specified, default to using email",
			spec: types.OIDCConnectorSpecV3{
				IssuerURL:     idp.S.URL,
				ClientID:      "000",
				ClientSecret:  "0000",
				ClaimsToRoles: []types.ClaimMapping{{Claim: "groups", Value: "everyone", Roles: []string{"access"}}},
				RedirectURLs:  []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
			},
			expectedUsername: "test-user@example.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			// Create OIDC connector with UsernameClaim specified.
			connector, err := types.NewOIDCConnector("okta-oidc", tc.spec)
			require.NoError(t, err)

			// Create OIDC request.
			oidcRequest := types.OIDCAuthRequest{
				ConnectorID:   "okta-oidc",
				Type:          constants.OIDC,
				CertTTL:       defaults.OIDCAuthRequestTTL,
				SSOTestFlow:   true,
				ConnectorSpec: &tc.spec,
			}
			request, err := s.a.CreateOIDCAuthRequest(ctx, oidcRequest)
			require.NoError(t, err)
			require.NotEmpty(t, request.RedirectURL)

			// Generate the userCreateParams for the OIDC user.
			createUserParams, err := s.oas.calculateOIDCUser(ctx, &diagCtx, connector, claims, ident, request)
			if tc.expectedError != "" {
				require.ErrorContains(t, err, tc.expectedError)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedUsername, createUserParams.Username)
			}
		})
	}
}

// TestReqMaxAge tests that MaxAge is correctly set in a OIDC authentication request.
func TestReqMaxAge(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := setUpSuite(t)
	idp := NewFakeOIDCIdP(t, false /* tls */)

	connectorSpec := types.OIDCConnectorSpecV3{
		IssuerURL:     idp.S.URL,
		ClientID:      "000",
		ClientSecret:  "0000",
		ClaimsToRoles: []types.ClaimMapping{{Claim: "groups", Value: "everyone", Roles: []string{"access"}}},
		RedirectURLs:  []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
		UsernameClaim: "preferred_username",
	}

	tests := []struct {
		name              string
		maxAge            *types.MaxAge
		expectedReqMaxAge string
		expectedErr       string
	}{
		{
			name:              "empty",
			maxAge:            nil,
			expectedReqMaxAge: "",
		},
		{
			name:              "zero",
			maxAge:            &types.MaxAge{Value: types.Duration(0)},
			expectedReqMaxAge: "0",
		},
		{
			name:              "hour",
			maxAge:            &types.MaxAge{Value: types.Duration(time.Hour)},
			expectedReqMaxAge: "3600",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := connectorSpec
			spec.MaxAge = tt.maxAge

			oidcRequest := types.OIDCAuthRequest{
				ConnectorID:   "okta-oidc",
				Type:          constants.OIDC,
				CertTTL:       defaults.OIDCAuthRequestTTL,
				SSOTestFlow:   true,
				ConnectorSpec: &spec,
			}
			request, err := s.a.CreateOIDCAuthRequest(ctx, oidcRequest)
			require.NoError(t, err)
			require.NotEmpty(t, request.RedirectURL)

			redirURL, err := url.Parse(request.RedirectURL)
			require.NoError(t, err)
			maxAge := redirURL.Query().Get("max_age")
			require.Equal(t, tt.expectedReqMaxAge, maxAge)
		})
	}
}

func TestValidateACRValues(t *testing.T) {
	tests := []struct {
		comment       string
		inIDToken     string
		inACRValue    string
		inACRProvider string
		outIsValid    require.ErrorAssertionFunc
	}{
		{
			"0 - default, acr values match",
			`
{
	"acr": "foo",
	"aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"foo",
			"",
			require.NoError,
		},
		{
			"1 - default, acr values do not match",
			`
{
	"acr": "foo",
	"aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"bar",
			"",
			require.Error,
		},
		{
			"2 - netiq, acr values match",
			`
{
    "acr": {
        "values": [
            "foo/bar/baz"
        ]
    },
    "aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"foo/bar/baz",
			"netiq",
			require.NoError,
		},
		{
			"3 - netiq, invalid format",
			`
{
    "acr": {
        "values": "foo/bar/baz"
    },
    "aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"foo/bar/baz",
			"netiq",
			require.Error,
		},
		{
			"4 - netiq, invalid value",
			`
{
    "acr": {
        "values": [
            "foo/bar/baz/qux"
        ]
    },
    "aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"foo/bar/baz",
			"netiq",
			require.Error,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.comment, func(t *testing.T) {
			t.Parallel()
			var claims jose.Claims
			err := json.Unmarshal([]byte(tt.inIDToken), &claims)
			require.NoError(t, err)

			err = validateACRValues(tt.inACRValue, tt.inACRProvider, claims)
			tt.outIsValid(t, err)
		})
	}
}

func TestOIDCAuthRequest(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			entitlements.OIDC: {Enabled: true},
		}},
	})

	ctx := context.Background()
	srv := newTestTLSServer(t, ValidLicense{})

	idp := NewFakeOIDCIdP(t, false /* tls */)

	emptyRole, err := auth.CreateRole(ctx, srv.Auth(), "test-empty", types.RoleSpecV6{})
	require.NoError(t, err)

	access1Role, err := auth.CreateRole(ctx, srv.Auth(), "test-access-1", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindOIDCRequest},
					Verbs:     []string{types.VerbCreate},
				},
			},
		},
	})
	require.NoError(t, err)

	access2Role, err := auth.CreateRole(ctx, srv.Auth(), "test-access-2", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindOIDC},
					Verbs:     []string{types.VerbCreate},
				},
			},
		},
	})
	require.NoError(t, err)

	access3Role, err := auth.CreateRole(ctx, srv.Auth(), "test-access-3", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindOIDC, types.KindOIDCRequest},
					Verbs:     []string{types.VerbCreate},
				},
			},
		},
	})
	require.NoError(t, err)

	readerRole, err := auth.CreateRole(ctx, srv.Auth(), "test-access-4", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindOIDCRequest},
					Verbs:     []string{types.VerbRead},
				},
			},
		},
	})
	require.NoError(t, err)

	conn, err := types.NewOIDCConnector("example", types.OIDCConnectorSpecV3{
		IssuerURL:    idp.S.URL,
		ClientID:     "example-client-id",
		ClientSecret: "example-client-secret",
		RedirectURLs: []string{"https://localhost:3080/v1/webapi/oidc/callback"},
		Display:      "sign in with example.com",
		Scope:        []string{"foo", "bar"},
		ClaimsToRoles: []types.ClaimMapping{
			{
				Claim: "groups",
				Value: "idp-admin",
				Roles: []string{"access"},
			},
		},
	})
	require.NoError(t, err)

	_, err = srv.Auth().CreateOIDCConnector(context.Background(), conn)
	require.NoError(t, err)

	reqNormal := types.OIDCAuthRequest{ConnectorID: conn.GetName(), Type: constants.OIDC}
	reqTest := types.OIDCAuthRequest{
		ConnectorID: conn.GetName(),
		Type:        constants.OIDC,
		SSOTestFlow: true,
		ConnectorSpec: &types.OIDCConnectorSpecV3{
			IssuerURL:    idp.S.URL,
			ClientID:     "example-client-id",
			ClientSecret: "example-client-secret",
			RedirectURLs: []string{"https://localhost:3080/v1/webapi/oidc/callback"},
			Display:      "sign in with example.com",
			Scope:        []string{"foo", "bar"},
			ClaimsToRoles: []types.ClaimMapping{
				{
					Claim: "groups",
					Value: "idp-admin",
					Roles: []string{"access"},
				},
			},
		},
	}

	tests := []struct {
		desc               string
		roles              []string
		request            types.OIDCAuthRequest
		expectAccessDenied bool
	}{
		{
			desc:               "empty role - no access",
			roles:              []string{emptyRole.GetName()},
			request:            reqNormal,
			expectAccessDenied: true,
		},
		{
			desc:               "can create regular request with normal access",
			roles:              []string{access1Role.GetName()},
			request:            reqNormal,
			expectAccessDenied: false,
		},
		{
			desc:               "cannot create sso test request with normal access",
			roles:              []string{access1Role.GetName()},
			request:            reqTest,
			expectAccessDenied: true,
		},
		{
			desc:               "cannot create normal request with connector access",
			roles:              []string{access2Role.GetName()},
			request:            reqNormal,
			expectAccessDenied: true,
		},
		{
			desc:               "cannot create sso test request with connector access",
			roles:              []string{access2Role.GetName()},
			request:            reqTest,
			expectAccessDenied: true,
		},
		{
			desc:               "can create regular request with combined access",
			roles:              []string{access3Role.GetName()},
			request:            reqNormal,
			expectAccessDenied: false,
		},
		{
			desc:               "can create sso test request with combined access",
			roles:              []string{access3Role.GetName()},
			request:            reqTest,
			expectAccessDenied: false,
		},
	}

	user, err := auth.CreateUser(ctx, srv.Auth(), "dummy")
	require.NoError(t, err)

	userReader, err := auth.CreateUser(ctx, srv.Auth(), "dummy-reader", readerRole)
	require.NoError(t, err)

	clientReader, err := srv.NewClient(auth.TestUser(userReader.GetName()))
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			user.SetRoles(tt.roles)
			user, err = srv.Auth().UpsertUser(ctx, user)
			require.NoError(t, err)

			client, err := srv.NewClient(auth.TestUser(user.GetName()))
			require.NoError(t, err)

			request, err := client.CreateOIDCAuthRequest(ctx, tt.request)
			if tt.expectAccessDenied {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err), "expected access denied, got: %v", err)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, request.StateToken)
			require.Equal(t, tt.request.ConnectorID, request.ConnectorID)

			requestCopy, err := clientReader.GetOIDCAuthRequest(ctx, request.StateToken)
			require.NoError(t, err)
			require.Equal(t, request, requestCopy)
		})
	}
}

// TestOIDCAuthCompat attempts to test OIDC SSO authentication from the
// perspective of an Auth service receiving requests from a proxy service. The
// Auth service on major version N should support proxies on version N and N-1,
// which may send a single user public key or split SSH and TLS public keys.
func TestOIDCAuthCompat(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			entitlements.OIDC: {Enabled: true},
		}},
	})

	ctx := context.Background()
	srv := newTestTLSServer(t, ValidLicense{}, func(cfg *auth.TestTLSServerConfig) {
		authPlugin, err := NewPlugin(Config{License: ValidLicense{}})
		require.NoError(t, err)
		reg := plugin.NewRegistry()
		reg.Add(authPlugin)
		cfg.APIConfig.PluginRegistry = reg
	})

	// There is no real OIDC IdP, override valid claims for a test user.
	SetStaticOIDCTestClaims(t, srv.Auth(), map[string]any{
		"groups": []string{"devs"},
		"email":  "alice@example.com",
		"sub":    "00001234abcd",
	})

	idp := NewFakeOIDCIdP(t, false /* tls */)

	conn, err := types.NewOIDCConnector("example", types.OIDCConnectorSpecV3{
		IssuerURL:    idp.S.URL,
		ClientID:     "example-client-id",
		ClientSecret: "example-client-secret",
		RedirectURLs: []string{"https://localhost:3080/v1/webapi/oidc/callback"},
		Display:      "sign in with example.com",
		Scope:        []string{"foo", "bar"},
		ClaimsToRoles: []types.ClaimMapping{
			{
				Claim: "groups",
				Value: "devs",
				Roles: []string{"access"},
			},
		},
	})
	require.NoError(t, err)

	_, err = srv.Auth().CreateOIDCConnector(context.Background(), conn)
	require.NoError(t, err)

	_, err = auth.CreateRole(ctx, srv.Auth(), "access", types.RoleSpecV6{})
	require.NoError(t, err)

	proxyClient, err := srv.NewClient(auth.TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	sshKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)
	sshPub, err := ssh.NewPublicKey(sshKey.Public())
	require.NoError(t, err)
	sshPubBytes := ssh.MarshalAuthorizedKey(sshPub)

	tlsKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	tlsPubBytes, err := keys.MarshalPublicKey(tlsKey.Public())
	require.NoError(t, err)

	for _, tc := range []struct {
		desc                         string
		pubKey, sshPubKey, tlsPubKey []byte
		expectSSHSubjectKey          ssh.PublicKey
		expectTLSSubjectKey          crypto.PublicKey
	}{
		{
			desc: "no keys",
		},
		{
			desc:                "single key",
			pubKey:              sshPubBytes,
			expectSSHSubjectKey: sshPub,
			expectTLSSubjectKey: sshKey.Public(),
		},
		{
			desc:                "split keys",
			sshPubKey:           sshPubBytes,
			tlsPubKey:           tlsPubBytes,
			expectSSHSubjectKey: sshPub,
			expectTLSSubjectKey: tlsKey.Public(),
		},
		{
			desc:                "only ssh",
			sshPubKey:           sshPubBytes,
			expectSSHSubjectKey: sshPub,
		},
		{
			desc:                "only tls",
			tlsPubKey:           tlsPubBytes,
			expectTLSSubjectKey: tlsKey.Public(),
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			req, err := proxyClient.CreateOIDCAuthRequest(ctx, types.OIDCAuthRequest{
				ConnectorID:  conn.GetName(),
				Type:         constants.OIDC,
				PublicKey:    tc.pubKey,
				SshPublicKey: tc.sshPubKey,
				TlsPublicKey: tc.tlsPubKey,
				CertTTL:      apidefaults.MinCertDuration,
				CheckUser:    true,
			})
			require.NoError(t, err, "creating OIDC auth request")

			values := url.Values{
				"code":  []string{"XXX-code"},
				"state": []string{req.StateToken},
			}
			resp, err := proxyClient.ValidateOIDCAuthCallback(ctx, values)
			require.NoError(t, err, "validating OIDC auth callback")

			// The proxy should get back the keys exactly as it sent them. Older
			// proxies won't look for the new split keys, and they do check for
			// the old single key to tell if this was a console or web request.
			require.Equal(t, tc.pubKey, resp.Req.PublicKey) //nolint:staticcheck // SA1019. Checking deprecated field expected by older clients.
			require.Equal(t, tc.sshPubKey, resp.Req.SSHPubKey)
			require.Equal(t, tc.tlsPubKey, resp.Req.TLSPubKey)

			// Make sure the subject key in the issued SSH cert matches the
			// expected key and didn't get accidentally switched.
			if tc.expectSSHSubjectKey != nil {
				sshCert, err := sshutils.ParseCertificate(resp.Cert)
				require.NoError(t, err)
				require.Equal(t, tc.expectSSHSubjectKey, sshCert.Key)
			} else {
				// No SSH cert should be issued if we didn't ask for one.
				require.Empty(t, resp.Cert)
			}

			// Make sure the subject key in the issued TLS cert matches the
			// expected key and didn't get accidentally switched.
			if tc.expectTLSSubjectKey != nil {
				tlsCert, err := tlsca.ParseCertificatePEM(resp.TLSCert)
				require.NoError(t, err)
				require.Equal(t, tc.expectTLSSubjectKey, tlsCert.PublicKey)
			} else {
				// No TLS cert should be issued if we didn't ask for one.
				require.Empty(t, resp.TLSCert)
			}
		})
	}
}

func TestOIDCLicense(t *testing.T) {
	idp := NewFakeOIDCIdP(t, false /* tls */)

	conn, err := types.NewOIDCConnector("example", types.OIDCConnectorSpecV3{
		IssuerURL:    idp.S.URL,
		ClientID:     "example-client-id",
		ClientSecret: "example-client-secret",
		RedirectURLs: []string{"https://localhost:3080/v1/webapi/oidc/callback"},
		Display:      "sign in with example.com",
		Scope:        []string{"foo", "bar"},
		ClaimsToRoles: []types.ClaimMapping{
			{
				Claim: "groups",
				Value: "idp-admin",
				Roles: []string{"access"},
			},
		},
	})
	require.NoError(t, err)

	tests := []struct {
		name        string
		license     License
		expectError bool
	}{
		{
			name:    "valid license",
			license: ValidLicense{},
		},
		{
			name:        "disabled license",
			license:     DisabledLicense{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			srv := newTestTLSServer(t, tt.license)
			_, err := srv.Auth().CreateOIDCConnector(ctx, conn)
			require.NoError(t, err)

			req := types.OIDCAuthRequest{ConnectorID: conn.GetName(), Type: constants.OIDC}
			_, err = srv.Auth().CreateOIDCAuthRequest(ctx, req)
			if tt.expectError {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err), "expected access denied, got: %v", err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestServer_ValidateOIDCResponse_MFA(t *testing.T) {
	ctx := context.Background()

	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			entitlements.OIDC: {Enabled: true},
		}},
	})

	srv := testserver.MakeTestServer(t)
	a := srv.GetAuthServer()

	mockEmitter := &eventstest.MockRecorderEmitter{}
	oas := registerOIDCService(t, &OIDCAuthServiceConfig{Auth: a, License: ValidLicense{}, Emitter: mockEmitter})

	idp := NewFakeOIDCIdP(t, false /* tls */)
	connectorName := "oidc-connector"
	conn, err := types.NewOIDCConnector(connectorName, types.OIDCConnectorSpecV3{
		IssuerURL:    idp.S.URL,
		ClientID:     "example-client-id",
		ClientSecret: "example-client-secret",
		RedirectURLs: []string{"https://localhost:3080/v1/webapi/oidc/callback"},
		Display:      "sign in with example.com",
		Scope:        []string{"foo", "bar"},
		ClaimsToRoles: []types.ClaimMapping{
			{
				Claim: "groups",
				Value: "idp-admin",
				Roles: []string{"access"},
			},
		},
		MFASettings: &types.OIDCConnectorMFASettings{
			Enabled:      true,
			ClientId:     "example-client-id",
			ClientSecret: "example-client-secret",
		},
	})
	require.NoError(t, err)

	_, err = a.CreateOIDCConnector(context.Background(), conn)
	require.NoError(t, err)

	request, err := oas.CreateOIDCAuthRequestForMFA(ctx, types.OIDCAuthRequest{
		ConnectorID: connectorName,
		Type:        constants.OIDC,
		CheckUser:   true,
	})
	require.NoError(t, err)

	// override getClaimsFun.
	username := "superuser@example.com"
	oas.getClaimsFun = func(closeCtx context.Context, oidcClient *oidc.Client, connector types.OIDCConnector, code string) (jose.Claims, error) {
		return map[string]any{
			"email_verified": true,
			"groups":         []string{"idp-admin"},
			"email":          username,
			"sub":            "00001234abcd",
			"exp":            float64(time.Now().Add(time.Hour).Unix()),
			// required since max_age=0.
			"auth_time": float64(time.Now().Unix()),
		}, nil
	}

	for _, tt := range []struct {
		name              string
		mutateSessionData func(sd *services.SSOMFASessionData)
		checkError        assert.ErrorAssertionFunc
		checkResponse     func(t *testing.T, resp *authclient.OIDCAuthResponse)
	}{
		{
			name:       "OK valid MFA session",
			checkError: assert.NoError,
			checkResponse: func(t *testing.T, resp *authclient.OIDCAuthResponse) {
				require.NotEmpty(t, resp)
				assert.NotZero(t, resp.MFAToken)

				// MFA session data token should match the response.
				sd, err := a.GetSSOMFASessionData(ctx, request.StateToken)
				assert.NoError(t, err)
				assert.Equal(t, resp.MFAToken, sd.Token)
			},
		},
		{
			name: "NOK username mismatch",
			mutateSessionData: func(sd *services.SSOMFASessionData) {
				sd.Username = "unknown"
			},
			checkError: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.True(t, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
			},
		},
		{
			name: "NOK connectorID mismatch",
			mutateSessionData: func(sd *services.SSOMFASessionData) {
				sd.ConnectorID = "unknown"
			},
			checkError: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.True(t, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
			},
		},
		{
			name: "NOK connectorType mismatch",
			mutateSessionData: func(sd *services.SSOMFASessionData) {
				sd.ConnectorType = "unknown"
			},
			checkError: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.True(t, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Add SSO MFA session data for the saml auth request. This should result in an MFA token being created.
			sd := &services.SSOMFASessionData{
				RequestID:     request.StateToken,
				Username:      username,
				ConnectorID:   connectorName,
				ConnectorType: constants.OIDC,
			}
			if tt.mutateSessionData != nil {
				tt.mutateSessionData(sd)
			}
			err = a.UpsertSSOMFASessionData(ctx, sd)
			require.NoError(t, err)

			// check ValidateSAMLResponse
			response, err := oas.ValidateOIDCAuthCallback(context.Background(), url.Values{
				"code":  []string{"XXX-code"},
				"state": []string{request.StateToken},
			})
			tt.checkError(t, err)

			if tt.checkResponse != nil {
				tt.checkResponse(t, response)
			}
		})
	}
}
