package saml

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/idp/saml/testenv"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestAuth(t *testing.T) {
	for _, tt := range []struct {
		// mfaQueryParamName is the name of the query parameter used for the MFA response.
		mfaQueryParamName string
	}{
		// TODO(cthach): DELETE IN v20.0.0 Webauthn query parameter.
		{mfaQueryParamName: Webauthn.String()},
		{mfaQueryParamName: MFAResponse.String()},
	} {
		ctx := context.Background()
		clock := clockwork.NewFakeClock()
		env := newTEnv(ctx, t, clock)
		env.testServices.Client.SigningCtx = testenv.WithRole(ctx, types.RoleProxy)

		user := setupUser(t, env.testServices, clock.Now().Add(time.Hour))

		path := path.Join(IdPRoute, "login", "shortcut-name")
		sp1, err := types.NewSAMLIdPServiceProvider(
			types.Metadata{
				Name: "shortcut-name",
			},
			types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: testenv.NewTestEntityDescriptor("sp1", "https://sp1.com/acs"),
				EntityID:         "sp1",
				RelayState:       "test-relay-state",
			},
		)
		require.NoError(t, err)
		require.NoError(t, env.testServices.SPService.CreateSAMLIdPServiceProvider(ctx, sp1))

		// No user.
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, path, nil)

		env.samlIdPService.ServeHTTP(w, r)
		require.Equal(t, http.StatusUnauthorized, w.Code)

		expectAuthAttemptEvent(t, env.testServices.Emitter, func(event *apievents.SAMLIdPAuthAttempt) {
			require.False(t, event.Success)
			require.Empty(t, event.User)
			require.Empty(t, event.ServiceProviderEntityID)
		})

		// User provided.
		w = httptest.NewRecorder()
		r = httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(authz.ContextWithUser(ctx, user))

		env.samlIdPService.ServeHTTP(w, r)
		require.Equal(t, http.StatusOK, w.Code)

		expectAuthAttemptEvent(t, env.testServices.Emitter, func(event *apievents.SAMLIdPAuthAttempt) {
			require.True(t, event.Success)
			require.Equal(t, user.Username, event.User)
			require.Empty(t, event.Error)
			require.Equal(t, "sp1", event.ServiceProviderEntityID)
		})

		// Disable access to the IdP.
		authPref, err := env.testServices.ClusterService.GetAuthPreference(ctx)
		require.NoError(t, err)
		authPref.SetSAMLIdPEnabled(false)
		authPref, err = env.testServices.ClusterService.UpdateAuthPreference(ctx, authPref)
		require.NoError(t, err)

		w = httptest.NewRecorder()
		r = httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(authz.ContextWithUser(ctx, user))

		env.samlIdPService.ServeHTTP(w, r)
		require.Equal(t, http.StatusForbidden, w.Code)

		expectAuthAttemptEvent(t, env.testServices.Emitter, func(event *apievents.SAMLIdPAuthAttempt) {
			require.False(t, event.Success)
			require.Equal(t, user.Username, event.User)
			require.Equal(t, "SAML IdP is disabled at the cluster level", event.Error)
			require.Equal(t, sp1.GetEntityID(), event.ServiceProviderEntityID)
		})

		// Reenable the SAML IdP.
		authPref.SetSAMLIdPEnabled(true)
		_, err = env.testServices.ClusterService.UpdateAuthPreference(ctx, authPref)
		require.NoError(t, err)

		// User is expired.
		user.Identity.Expires = clock.Now().Add(-30 * time.Minute)
		w = httptest.NewRecorder()
		r = httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(authz.ContextWithUser(ctx, user))

		env.samlIdPService.ServeHTTP(w, r)
		require.Equal(t, http.StatusUnauthorized, w.Code)

		expectAuthAttemptEvent(t, env.testServices.Emitter, func(event *apievents.SAMLIdPAuthAttempt) {
			require.False(t, event.Success)
			require.Equal(t, user.Username, event.User)
			require.Contains(t, event.Error, "identity is expired")
			require.Empty(t, event.ServiceProviderEntityID)
		})

		user.Identity.Expires = user.Identity.Expires.Add(30 * time.Minute)

		// Require MFA.
		a, ok := authPref.(*types.AuthPreferenceV2)
		require.True(t, ok)
		a.Spec.RequireMFAType = types.RequireMFAType_SESSION
		_, err = env.testServices.ClusterService.UpdateAuthPreference(ctx, authPref)
		require.NoError(t, err)

		// No MFA should redirect to /web/saml-idp/login.
		w = httptest.NewRecorder()
		r = httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(authz.ContextWithUser(ctx, user))

		env.samlIdPService.ServeHTTP(w, r)
		require.Equal(t, http.StatusSeeOther, w.Code)

		// no event is emitted as this redirect is expected.

		result := w.Result()
		require.NoError(t, result.Body.Close())
		resultURL, err := result.Location()
		require.NoError(t, err)
		require.Equal(t, "/web/saml-idp/login", resultURL.Path)
		require.Equal(t, "redirect_uri=https://example.com/enterprise/saml-idp/login/shortcut-name", resultURL.RawQuery)

		// MFA provided.
		w = httptest.NewRecorder()
		r = httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(authz.ContextWithUser(ctx, user))

		fakeWebauthnResponse := &client.MFAChallengeResponse{
			WebauthnAssertionResponse: &webauthntypes.CredentialAssertionResponse{
				PublicKeyCredential: webauthntypes.PublicKeyCredential{
					Credential: webauthntypes.Credential{
						ID:   "id",
						Type: "type",
					},
				},
			},
		}
		webauthnBytes, err := json.Marshal(fakeWebauthnResponse)
		require.NoError(t, err)
		r.URL.RawQuery = url.Values{
			tt.mfaQueryParamName: []string{base64.RawURLEncoding.EncodeToString(webauthnBytes)},
		}.Encode()

		env.samlIdPService.ServeHTTP(w, r)
		require.Equal(t, http.StatusOK, w.Code)

		expectAuthAttemptEvent(t, env.testServices.Emitter, func(event *apievents.SAMLIdPAuthAttempt) {
			require.True(t, event.Success)
			require.Equal(t, user.Username, event.User)
			require.Empty(t, event.Error)
			require.Equal(t, "sp1", event.ServiceProviderEntityID)
		})
	}
}

func TestMetadata(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	env := newTEnv(ctx, t, clock)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, path.Join(IdPRoute, "metadata"), nil)

	env.samlIdPService.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	ed := saml.EntityDescriptor{}
	require.NoError(t, xml.Unmarshal(w.Body.Bytes(), &ed))
	require.Equal(t, fmt.Sprintf("https://test.url%s", path.Join(IdPRoute, "metadata")), ed.EntityID)
}

func TestMetadataValues(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	env := newTEnv(ctx, t, clock)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, path.Join(IdPRoute, "metadata-values"), nil)

	env.samlIdPService.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	var metadata idpMetadataValues
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &metadata))
	require.Equal(t, fmt.Sprintf("https://test.url%s", path.Join(IdPRoute, "metadata")), metadata.EntityID)
	require.Equal(t, fmt.Sprintf("https://test.url%s", path.Join(IdPRoute, "sso")), metadata.SSOURL)

	// test that certificate from metadata matches CA cert that is used to
	// sign SAML assertion.
	clusterName, err := env.testServices.ClusterService.GetClusterName(ctx)
	require.NoError(t, err)
	ca, err := env.testServices.CAService.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.SAMLIDPCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	require.NoError(t, err)
	rawCert, _, err := env.testServices.KeyStore.GetTLSCertAndSigner(ctx, ca)
	require.NoError(t, err)
	require.Equal(t, string(rawCert), metadata.X509PEM)
}

func TestCreateIdPError(t *testing.T) {
	ctx := t.Context()
	clock := clockwork.NewFakeClock()
	env := newTEnv(ctx, t, clock)
	env.testServices.Client.SigningCtx = testenv.WithRole(ctx, types.RoleProxy)

	user := setupUser(t, env.testServices, clock.Now().Add(time.Hour))

	sp1, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{Name: "sp1"},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: testenv.NewTestEntityDescriptor("sp1", "https://sp1.com/acs"),
			EntityID:         "sp1",
		},
	)
	require.NoError(t, err)
	require.NoError(t, env.testServices.SPService.CreateSAMLIdPServiceProvider(ctx, sp1))

	// Delete the SAML IdP CA to induce failures.
	require.NoError(t, env.testServices.CAService.DeleteCertAuthority(ctx, types.CertAuthID{
		Type:       types.SAMLIDPCA,
		DomainName: "test-cluster",
	}))

	tests := []struct {
		name     string
		method   string
		path     string
		needAuth bool
	}{
		{name: "metadata", method: http.MethodGet, path: path.Join(IdPRoute, "metadata")},
		{name: "metadata-values", method: http.MethodGet, path: path.Join(IdPRoute, "metadata-values")},
		{name: "sso GET", method: http.MethodGet, path: path.Join(IdPRoute, "sso"), needAuth: true},
		{name: "sso POST", method: http.MethodPost, path: path.Join(IdPRoute, "sso"), needAuth: true},
		{name: "idp-initiated login", method: http.MethodGet, path: path.Join(IdPRoute, "login", sp1.GetName()), needAuth: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.needAuth {
				r = r.WithContext(authz.ContextWithUser(r.Context(), user))
			}
			env.samlIdPService.ServeHTTP(w, r)
			require.Equal(t, http.StatusForbidden, w.Code)
		})
	}
}

func TestSSOGET(t *testing.T) {
	testSSO(t, http.MethodGet)
}

func TestSSOPOST(t *testing.T) {
	testSSO(t, http.MethodPost)
}

// Note: The XML validator will return error on a valid tag supplied to ACS field,
// so for test purpose, only a greater-than character is used below.
// The payload itself is not meant for an exhaustive string escaping test,
// we only want to check the characters are being escaped
// in sensitive fields as expected.
const acsURLWithHTMLTag = `https://sp.com>script>`
const relayStateWithHTMlTag = `"<script>"`

func createSamlIdPServiceProviderItem(t *testing.T, ctx context.Context, env *tEnvWithSAMLService) types.SAMLIdPServiceProvider {
	sp1, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp1",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: testenv.NewTestEntityDescriptor("sp1", acsURLWithHTMLTag),
			EntityID:         "sp1",
			RelayState:       relayStateWithHTMlTag,
		},
	)
	require.NoError(t, err)
	// manually creating resource as we no longer allow certain HTML tag characters in the
	// CreateSAMLIdPServiceProvider, UpdateSAMLIdPServiceProvider methods and require the
	// XML to be a valid XML format.
	item, err := env.testServices.GenericService.MakeBackendItem(sp1, sp1.GetName())
	require.NoError(t, err)
	_, err = env.testServices.UserService.Backend.Create(ctx, item)
	require.NoError(t, err)

	return sp1
}

func testSSO(t *testing.T, method string) {
	ctx := context.Background()
	clock := clockwork.NewRealClock()
	env := newTEnv(ctx, t, clock)
	env.testServices.Client.SigningCtx = testenv.WithRole(ctx, types.RoleProxy)

	user := setupUser(t, env.testServices, clock.Now().Add(time.Hour))

	createSamlIdPServiceProviderItem(t, ctx, env)

	authnRequest := saml.AuthnRequest{
		ID:           "auth-id",
		Version:      "2.0",
		IssueInstant: clock.Now(),
		Issuer: &saml.Issuer{
			Value: "sp1",
		},
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, path.Join(IdPRoute, "sso"), nil)
	r = r.WithContext(authz.ContextWithUser(r.Context(), user))

	authnMessage := testenv.MakeAuthnMessage(t, authnRequest, method, relayStateWithHTMlTag)
	if method == http.MethodGet {
		r.URL.RawQuery = authnMessage.Encode()
	} else {
		r.PostForm = authnMessage
	}

	env.samlIdPService.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	expectAuthAttemptEvent(t, env.testServices.Emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.True(t, event.Success)
		require.Equal(t, user.Username, event.User)
		require.Empty(t, event.Error)
		require.Equal(t, "sp1", event.ServiceProviderEntityID)
	})

	// csp header validation
	cspStr := w.Header().Get("Content-Security-Policy")
	csp, err := parseCSP(cspStr)
	require.NoError(t, err)
	valdiateBaseCSPValues(t, csp)
	nonceHexFromScriptDirective := nonceHexValue(csp["script-src"])
	require.NotEmpty(t, nonceHexFromScriptDirective)

	// html.Parse will escape the HTML tags, so below we manually check if the
	// relayStateWithHTMlTag string is escaped. Test value is hardcoded below for
	// readability.
	require.Contains(t, w.Body.String(), `&#34;&lt;script&gt;&#34;`)

	node, err := html.Parse(w.Body)
	require.NoError(t, err)

	formNode := testenv.FindNode(node, "form")
	require.NotNil(t, formNode)
	require.Equal(t, "method", formNode.Attr[0].Key)
	require.Equal(t, "post", formNode.Attr[0].Val)
	require.Equal(t, "action", formNode.Attr[1].Key)
	require.Equal(t, `https://sp.com%3escript%3e`, formNode.Attr[1].Val)
	require.Equal(t, "id", formNode.Attr[2].Key)
	require.Equal(t, "SAMLResponseForm", formNode.Attr[2].Val)

	inputNode := testenv.FindNode(formNode, "input")
	require.Equal(t, SAMLResponse.String(), inputNode.Attr[1].Val)
	require.Equal(t, RelayState.String(), inputNode.NextSibling.NextSibling.Attr[1].Val)
	require.Equal(t, html.UnescapeString(relayStateWithHTMlTag), inputNode.NextSibling.NextSibling.Attr[2].Val)

	// compare csp values from header and script tags.
	scriptNode := testenv.FindNode(node, "script")
	require.NotNil(t, scriptNode)
	require.Equal(t, scriptNode.Attr[0].Val, nonceHexFromScriptDirective)
}

func TestIdPInitiatedLoginGET(t *testing.T) {
	testIdPInitiatedLogin(t, http.MethodGet)
}

func TestIdPInitiatedLoginPOST(t *testing.T) {
	testIdPInitiatedLogin(t, http.MethodPost)
}

//nolint:revive // Because we want this to be IdP.
func testIdPInitiatedLogin(t *testing.T, method string) {
	ctx := context.Background()
	clock := clockwork.NewRealClock()
	env := newTEnv(ctx, t, clock)
	env.testServices.Client.SigningCtx = testenv.WithRole(ctx, types.RoleProxy)

	user := setupUser(t, env.testServices, clock.Now().Add(time.Hour))

	sp1 := createSamlIdPServiceProviderItem(t, ctx, env)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, path.Join(IdPRoute, "login", sp1.GetName()), nil)
	r = r.WithContext(authz.ContextWithUser(r.Context(), user))

	env.samlIdPService.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	expectAuthAttemptEvent(t, env.testServices.Emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.True(t, event.Success)
		require.Equal(t, user.Username, event.User)
		require.Empty(t, event.Error)
		require.Equal(t, "sp1", event.ServiceProviderEntityID)
	})

	cspStr := w.Header().Get("Content-Security-Policy")
	csp, err := parseCSP(cspStr)
	require.NoError(t, err)
	valdiateBaseCSPValues(t, csp)
	nonceHexFromScriptDirective := nonceHexValue(csp["script-src"])
	require.NotEmpty(t, nonceHexFromScriptDirective)

	// html.Parse will escape the HTML tags, so below we manually check if the
	// relayStateWithHTMlTag string is escaped. Value is hardcoded below for
	// readability of test value.
	require.Contains(t, w.Body.String(), `&#34;&lt;script&gt;&#34;` /* HTML string escaped value of relayStateWithHTMlTag */)

	node, err := html.Parse(w.Body)
	require.NoError(t, err)

	formNode := testenv.FindNode(node, "form")
	require.NotNil(t, formNode)
	require.Equal(t, "method", formNode.Attr[0].Key)
	require.Equal(t, "post", formNode.Attr[0].Val)
	require.Equal(t, "action", formNode.Attr[1].Key)
	require.Equal(t, `https://sp.com%3escript%3e`, formNode.Attr[1].Val)
	require.Equal(t, "id", formNode.Attr[2].Key)
	require.Equal(t, "SAMLResponseForm", formNode.Attr[2].Val)

	inputNode := testenv.FindNode(formNode, "input")
	require.Equal(t, SAMLResponse.String(), inputNode.Attr[1].Val)
	require.Equal(t, RelayState.String(), inputNode.NextSibling.NextSibling.Attr[1].Val)
	require.Equal(t, html.UnescapeString(relayStateWithHTMlTag), inputNode.NextSibling.NextSibling.Attr[2].Val)
	// require.Equal(t, html.UnescapeString(relayStateWithHTMlTag), formNode.FirstChild.NextSibling.Attr[2].Val)

	// compare csp values from header and script tags.
	scriptNode := testenv.FindNode(node, "script")
	require.NotNil(t, scriptNode)
	require.Equal(t, scriptNode.Attr[0].Val, nonceHexFromScriptDirective)

	w = httptest.NewRecorder()
	r = httptest.NewRequest(method, path.Join(IdPRoute, "login/doesntexist"), nil)
	r = r.WithContext(authz.ContextWithUser(r.Context(), user))

	env.samlIdPService.ServeHTTP(w, r)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "You do not have access to this resource")

	expectAuthAttemptEvent(t, env.testServices.Emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.False(t, event.Success)
		require.Equal(t, user.Username, event.User)
		require.Equal(t, "saml_idp_service_provider \"doesntexist\" doesn't exist", event.Error)
		require.Empty(t, "", event.ServiceProviderEntityID)
		require.Equal(t, "doesntexist", event.ServiceProviderShortcut)
	})
}

func TestLockUser(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	env := newTEnv(ctx, t, clock)

	user := setupUser(t, env.testServices, clock.Now().Add(time.Hour))
	path := path.Join(IdPRoute, "sso")

	lock, err := types.NewLock("test-lock", types.LockSpecV2{
		Target: types.LockTarget{
			User: user.Username,
		},
	})
	require.NoError(t, err)
	env.testServices.AccessService.UpsertLock(ctx, lock)

	// After adding the lock, the GET should fail.
	require.Eventually(t, func() bool {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(authz.ContextWithUser(r.Context(), user))
		env.samlIdPService.ServeHTTP(w, r)
		return w.Code == http.StatusUnauthorized
	}, time.Second*3, time.Millisecond*250)
}

func TestIdPInitiatedSSOWithRBAC(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewRealClock()
	env := newTEnv(ctx, t, clock)
	env.testServices.Client.SigningCtx = testenv.WithRole(ctx, types.RoleProxy)
	user := setupUser(t, env.testServices, clock.Now().Add(time.Hour))

	sp1, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "saml-app",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: testenv.NewTestEntityDescriptor("saml-app", "https://saml-app/acs"),
			EntityID:         "saml-app",
			RelayState:       "test-relay-state",
		},
	)
	require.NoError(t, err)
	require.NoError(t, env.testServices.SPService.CreateSAMLIdPServiceProvider(ctx, sp1))

	tests := []struct {
		name               string
		allow              types.RoleConditions
		deny               types.RoleConditions
		options            types.RoleOptions
		isDeviceAuthorized bool
		httpStatus         int
		authAttemptEvent   func(*apievents.SAMLIdPAuthAttempt)
	}{
		{
			name:       "without app label",
			allow:      types.RoleConditions{},
			httpStatus: http.StatusForbidden,
			authAttemptEvent: func(event *apievents.SAMLIdPAuthAttempt) {
				require.False(t, event.Success)
				require.Equal(t, user.Username, event.User)
				require.Contains(t, event.Error, "User does not have permissions")
				require.Equal(t, sp1.GetEntityID(), event.ServiceProviderEntityID)
			},
		},
		{
			name: "with app label but denied saml_idp_service_provider resource read verb",
			allow: types.RoleConditions{
				AppLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			},
			deny: types.RoleConditions{
				Rules: []types.Rule{
					{
						Resources: []string{types.KindSAMLIdPServiceProvider},
						Verbs:     services.RO(),
					},
				},
			},
			httpStatus: http.StatusForbidden,
			authAttemptEvent: func(event *apievents.SAMLIdPAuthAttempt) {
				require.False(t, event.Success)
				require.Equal(t, user.Username, event.User)
				require.Contains(t, event.Error, "access denied")
				require.Equal(t, sp1.GetEntityID(), event.ServiceProviderEntityID)
			},
		},
		{
			name: "with wildcard app label",
			allow: types.RoleConditions{
				AppLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				Rules: []types.Rule{
					{
						Resources: []string{types.KindSAMLIdPServiceProvider},
						Verbs:     services.RO(),
					},
				},
			},
			httpStatus: http.StatusOK,
			authAttemptEvent: func(event *apievents.SAMLIdPAuthAttempt) {
				require.True(t, event.Success)
				require.Equal(t, user.Username, event.User)
				require.Empty(t, event.Error)
				require.Equal(t, sp1.GetEntityID(), event.ServiceProviderEntityID)
			},
		},
		{
			name: "with device trust required and no authorized device",
			allow: types.RoleConditions{
				AppLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				Rules: []types.Rule{
					{
						Resources: []string{types.KindSAMLIdPServiceProvider},
						Verbs:     services.RO(),
					},
				},
			},
			options: types.RoleOptions{
				DeviceTrustMode: "required",
			},
			isDeviceAuthorized: false,
			httpStatus:         http.StatusForbidden,
			authAttemptEvent: func(event *apievents.SAMLIdPAuthAttempt) {
				require.False(t, event.Success)
				require.Equal(t, user.Username, event.User)
				require.Contains(t, event.Error, "requires a trusted device")
				require.Equal(t, sp1.GetEntityID(), event.ServiceProviderEntityID)
			},
		},
		{
			name: "with device trust required and authorized device",
			allow: types.RoleConditions{
				AppLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				Rules: []types.Rule{
					{
						Resources: []string{types.KindSAMLIdPServiceProvider},
						Verbs:     services.RO(),
					},
				},
			},
			options: types.RoleOptions{
				DeviceTrustMode: "required",
			},
			isDeviceAuthorized: true,
			httpStatus:         http.StatusOK,
			authAttemptEvent: func(event *apievents.SAMLIdPAuthAttempt) {
				require.True(t, event.Success)
				require.Equal(t, user.Username, event.User)
				require.Empty(t, event.Error)
				require.Equal(t, sp1.GetEntityID(), event.ServiceProviderEntityID)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			role, err := env.testServices.AccessService.GetRole(ctx, roleName)
			require.NoError(t, err)

			role.SetRules(types.Allow, test.allow.Rules)
			role.SetRules(types.Deny, test.deny.Rules)
			role.SetAppLabels(types.Allow, test.allow.AppLabels)
			role.SetOptions(test.options)
			_, err = env.testServices.AccessService.UpsertRole(ctx, role)
			require.NoError(t, err)
			if test.isDeviceAuthorized {
				user.Identity.DeviceExtensions = tlsca.DeviceExtensions{
					DeviceID:     "abc123",
					AssetTag:     "abc123",
					CredentialID: "abc123",
				}
			}

			w := httptest.NewRecorder()
			// try with Get
			r := httptest.NewRequest(http.MethodGet, path.Join(IdPRoute, "login", "saml-app"), nil)
			r = r.WithContext(authz.ContextWithUser(r.Context(), user))

			env.samlIdPService.ServeHTTP(w, r)
			require.Equal(t, test.httpStatus, w.Code)
			expectAuthAttemptEvent(t, env.testServices.Emitter, test.authAttemptEvent)

			// try with POST
			r = httptest.NewRequest(http.MethodPost, path.Join(IdPRoute, "login", "saml-app"), nil)
			r = r.WithContext(authz.ContextWithUser(r.Context(), user))

			env.samlIdPService.ServeHTTP(w, r)
			require.Equal(t, test.httpStatus, w.Code)
			expectAuthAttemptEvent(t, env.testServices.Emitter, test.authAttemptEvent)

			if test.httpStatus == http.StatusForbidden {
				if test.options.DeviceTrustMode == "required" {
					require.Contains(t, w.Body.String(), "Trusted Device")
				} else {
					require.Contains(t, w.Body.String(), "You do not have access to this resource")
				}
			}
		})
	}
}

func TestSPInitiatedSSOWithRBAC(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewRealClock()
	env := newTEnv(ctx, t, clock)
	env.testServices.Client.SigningCtx = testenv.WithRole(ctx, types.RoleProxy)
	user := setupUser(t, env.testServices, clock.Now().Add(time.Hour))

	sp1, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "saml-app",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: testenv.NewTestEntityDescriptor("saml-app", "https://saml-app/acs"),
			EntityID:         "saml-app",
			RelayState:       "test-relay-state",
		},
	)
	require.NoError(t, err)
	require.NoError(t, env.testServices.SPService.CreateSAMLIdPServiceProvider(ctx, sp1))

	tests := []struct {
		name               string
		allow              types.RoleConditions
		deny               types.RoleConditions
		options            types.RoleOptions
		isDeviceAuthorized bool
		httpStatus         int
		authAttemptEvent   func(*apievents.SAMLIdPAuthAttempt)
	}{
		{
			name:       "without app label and saml_idp_service_provider resource read verb",
			allow:      types.RoleConditions{},
			httpStatus: http.StatusForbidden,
			authAttemptEvent: func(event *apievents.SAMLIdPAuthAttempt) {
				require.False(t, event.Success)
				require.Equal(t, user.Username, event.User)
				require.Contains(t, event.Error, "User does not have permissions")
				require.Equal(t, sp1.GetEntityID(), event.ServiceProviderEntityID)
			},
		},
		{
			name: "with app label but denied saml_idp_service_provider resource read verb",
			allow: types.RoleConditions{
				AppLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			},
			deny: types.RoleConditions{
				Rules: []types.Rule{
					{
						Resources: []string{types.KindSAMLIdPServiceProvider},
						Verbs:     services.RO(),
					},
				},
			},
			httpStatus: http.StatusForbidden,
			authAttemptEvent: func(event *apievents.SAMLIdPAuthAttempt) {
				require.False(t, event.Success)
				require.Equal(t, user.Username, event.User)
				require.Contains(t, event.Error, "access denied")
				require.Equal(t, sp1.GetEntityID(), event.ServiceProviderEntityID)
			},
		},
		{
			name: "with read verb and wildcard app label",
			allow: types.RoleConditions{
				AppLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				Rules: []types.Rule{
					{
						Resources: []string{types.KindSAMLIdPServiceProvider},
						Verbs:     services.RO(),
					},
				},
			},
			httpStatus: http.StatusOK,
			authAttemptEvent: func(event *apievents.SAMLIdPAuthAttempt) {
				require.True(t, event.Success)
				require.Equal(t, user.Username, event.User)
				require.Empty(t, event.Error)
				require.Equal(t, sp1.GetEntityID(), event.ServiceProviderEntityID)
			},
		},
		{
			name: "with device trust required and no authorized device",
			allow: types.RoleConditions{
				AppLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				Rules: []types.Rule{
					{
						Resources: []string{types.KindSAMLIdPServiceProvider},
						Verbs:     services.RO(),
					},
				},
			},
			options: types.RoleOptions{
				DeviceTrustMode: "required",
			},
			isDeviceAuthorized: false,
			httpStatus:         http.StatusForbidden,
			authAttemptEvent: func(event *apievents.SAMLIdPAuthAttempt) {
				require.False(t, event.Success)
				require.Equal(t, user.Username, event.User)
				require.Contains(t, event.Error, "requires a trusted device")
				require.Equal(t, sp1.GetEntityID(), event.ServiceProviderEntityID)
			},
		},
		{
			name: "with device trust required and authorized device",
			allow: types.RoleConditions{
				AppLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				Rules: []types.Rule{
					{
						Resources: []string{types.KindSAMLIdPServiceProvider},
						Verbs:     services.RO(),
					},
				},
			},
			options: types.RoleOptions{
				DeviceTrustMode: "required",
			},
			isDeviceAuthorized: true,
			httpStatus:         http.StatusOK,
			authAttemptEvent: func(event *apievents.SAMLIdPAuthAttempt) {
				require.True(t, event.Success)
				require.Equal(t, user.Username, event.User)
				require.Empty(t, event.Error)
				require.Equal(t, sp1.GetEntityID(), event.ServiceProviderEntityID)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			role, err := env.testServices.AccessService.GetRole(ctx, roleName)
			require.NoError(t, err)

			role.SetRules(types.Allow, test.allow.Rules)
			role.SetRules(types.Deny, test.deny.Rules)
			role.SetAppLabels(types.Allow, test.allow.AppLabels)
			role.SetOptions(test.options)
			_, err = env.testServices.AccessService.UpsertRole(ctx, role)
			require.NoError(t, err)
			if test.isDeviceAuthorized {
				user.Identity.DeviceExtensions = tlsca.DeviceExtensions{
					DeviceID:     "abc123",
					AssetTag:     "abc123",
					CredentialID: "abc123",
				}
			}

			authnRequest := saml.AuthnRequest{
				ID:           "auth-id",
				Version:      "2.0",
				IssueInstant: clock.Now(),
				Issuer: &saml.Issuer{
					Value: sp1.GetEntityID(),
				},
			}
			w := httptest.NewRecorder()

			// HTTP-POST binding
			r := httptest.NewRequest(http.MethodPost, path.Join(IdPRoute, "sso"), nil)
			r = r.WithContext(authz.ContextWithUser(r.Context(), user))
			r.PostForm = testenv.MakeAuthnMessage(t, authnRequest, http.MethodPost, "")

			env.samlIdPService.ServeHTTP(w, r)
			require.Equal(t, test.httpStatus, w.Code)
			expectAuthAttemptEvent(t, env.testServices.Emitter, test.authAttemptEvent)

			// HTTP-Redirect binding
			r = httptest.NewRequest(http.MethodGet, path.Join(IdPRoute, "sso"), nil)
			r = r.WithContext(authz.ContextWithUser(r.Context(), user))
			r.URL.RawQuery = testenv.MakeAuthnMessage(t, authnRequest, http.MethodGet, "").Encode()

			env.samlIdPService.ServeHTTP(w, r)
			require.Equal(t, test.httpStatus, w.Code)
			expectAuthAttemptEvent(t, env.testServices.Emitter, test.authAttemptEvent)

			if test.httpStatus == http.StatusForbidden {
				if test.options.DeviceTrustMode == "required" {
					require.Contains(t, w.Body.String(), "Trusted Device")
				} else {
					require.Contains(t, w.Body.String(), "You do not have access to this resource")
				}
			}
		})
	}
}

const roleName = "test-role"

// setupUser creates user with specified role. If roleSpec is nil, default role is applied
// that has an Allow condition with a wildcard App Label and rule for KindSAMLIdPServiceProvider
// resource with services.RO() verbs.
func setupUser(t *testing.T, svcs testenv.TEnv, expireTime time.Time) authz.LocalUser {
	ctx := context.Background()
	clt := struct {
		services.Access
		services.Identity
	}{
		Access:   svcs.AccessService,
		Identity: svcs.UserService,
	}

	role, err := authtest.CreateRole(ctx, clt, roleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			Rules: []types.Rule{
				{
					Resources: []string{types.KindSAMLIdPServiceProvider},
					Verbs:     services.RO(),
				},
			},
		},
	})
	require.NoError(t, err)

	user, err := types.NewUser("user1")
	user.AddRole(role.GetName())
	require.NoError(t, err)
	user, err = svcs.UserService.CreateUser(ctx, user)
	require.NoError(t, err)

	identity := tlsca.Identity{
		Username: user.GetName(),
		Groups:   []string{roleName},
		Expires:  expireTime,
	}

	return authz.LocalUser{
		Username: user.GetName(),
		Identity: identity,
	}
}
