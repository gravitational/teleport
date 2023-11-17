package saml

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/base64"
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
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestAuth(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	svcs := samlTestService(ctx, t, clock)

	user := setupUser(t, svcs, clock.Now().Add(time.Hour))

	// No user.
	path := path.Join(IdPRoute, "metadata")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)

	svcs.samlIdP.ServeHTTP(w, r)
	require.Equal(t, http.StatusNotFound, w.Code)

	expectAuthAttemptEvent(t, svcs.emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.False(t, event.Success)
		require.Empty(t, event.User)
		require.Empty(t, event.ServiceProviderEntityID)
	})

	// Disable access to the IdP.
	authPref, err := svcs.clusterService.GetAuthPreference(ctx)
	require.NoError(t, err)
	authPref.SetSAMLIdPEnabled(false)
	require.NoError(t, svcs.clusterService.SetAuthPreference(ctx, authPref))

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, path, nil)
	r = r.WithContext(authz.ContextWithUser(r.Context(), user))

	svcs.samlIdP.ServeHTTP(w, r)
	require.Equal(t, http.StatusNotFound, w.Code)

	expectAuthAttemptEvent(t, svcs.emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.False(t, event.Success)
		require.Equal(t, user.Username, event.User)
		require.Equal(t, "SAML IdP is disabled at the cluster level", event.Error)
		require.Empty(t, event.ServiceProviderEntityID)
	})

	// Reenable the SAML IdP.
	authPref.SetSAMLIdPEnabled(true)
	require.NoError(t, svcs.clusterService.SetAuthPreference(ctx, authPref))

	w = httptest.NewRecorder()

	svcs.samlIdP.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	// User is expired.
	user.Identity.Expires = clock.Now().Add(-30 * time.Minute)
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, path, nil)
	r = r.WithContext(authz.ContextWithUser(r.Context(), user))

	svcs.samlIdP.ServeHTTP(w, r)
	require.Equal(t, http.StatusNotFound, w.Code)

	expectAuthAttemptEvent(t, svcs.emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.False(t, event.Success)
		require.Equal(t, user.Username, event.User)
		require.Equal(t, "identity is expired", event.Error)
		require.Empty(t, event.ServiceProviderEntityID)
	})
}

func TestMetadata(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	svcs := samlTestService(ctx, t, clock)

	user := setupUser(t, svcs, clock.Now().Add(time.Hour))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, path.Join(IdPRoute, "metadata"), nil)
	r = r.WithContext(authz.ContextWithUser(r.Context(), user))

	svcs.samlIdP.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	ed := saml.EntityDescriptor{}
	require.NoError(t, xml.Unmarshal(w.Body.Bytes(), &ed))
	require.Equal(t, fmt.Sprintf("https://test.url%s", path.Join(IdPRoute, "metadata")), ed.EntityID)
}

func TestSSOGET(t *testing.T) {
	testSSO(t, http.MethodGet, func(r *http.Request, authnRequest saml.AuthnRequest) {
		var buf bytes.Buffer
		require.NoError(t, xml.NewEncoder(&buf).Encode(authnRequest))

		var compressedBuf bytes.Buffer
		flateWriter, err := flate.NewWriter(&compressedBuf, flate.DefaultCompression)
		flateWriter.Write(buf.Bytes())
		require.NoError(t, flateWriter.Close())

		encodedRequest := base64.StdEncoding.EncodeToString(compressedBuf.Bytes())
		require.NoError(t, err)

		values := r.URL.Query()
		values.Add("SAMLRequest", encodedRequest)
		r.URL.RawQuery = values.Encode()
	})
}

func TestSSOPOST(t *testing.T) {
	testSSO(t, http.MethodPost, func(r *http.Request, authnRequest saml.AuthnRequest) {
		var buf bytes.Buffer
		require.NoError(t, xml.NewEncoder(&buf).Encode(authnRequest))

		encodedRequest := base64.StdEncoding.EncodeToString(buf.Bytes())

		r.PostForm = url.Values{}
		r.PostForm.Add("SAMLRequest", encodedRequest)
	})
}

func testSSO(t *testing.T, method string, addRequest func(*http.Request, saml.AuthnRequest)) {
	ctx := context.Background()
	clock := clockwork.NewRealClock()
	svcs := samlTestService(ctx, t, clock)
	svcs.client.signingCtx = withRole(ctx, types.RoleProxy)

	user := setupUser(t, svcs, clock.Now().Add(time.Hour))

	sp1, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp1",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newTestEntityDescriptor("sp1"),
			EntityID:         "sp1",
		},
	)
	require.NoError(t, err)
	require.NoError(t, svcs.spService.CreateSAMLIdPServiceProvider(ctx, sp1))

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

	addRequest(r, authnRequest)

	svcs.samlIdP.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	expectAuthAttemptEvent(t, svcs.emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.True(t, event.Success)
		require.Equal(t, user.Username, event.User)
		require.Empty(t, event.Error)
		require.Equal(t, "sp1", event.ServiceProviderEntityID)
	})

	node, err := html.Parse(w.Body)
	require.NoError(t, err)

	formNode := findNode(node, "form")
	require.NotNil(t, formNode)
	require.Equal(t, "method", formNode.Attr[0].Key)
	require.Equal(t, "post", formNode.Attr[0].Val)
	require.Equal(t, "action", formNode.Attr[1].Key)
	require.Equal(t, "https://sptest.iamshowcase.com/acs", formNode.Attr[1].Val)
	require.Equal(t, "id", formNode.Attr[2].Key)
	require.Equal(t, "SAMLResponseForm", formNode.Attr[2].Val)
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
	svcs := samlTestService(ctx, t, clock)
	svcs.client.signingCtx = withRole(ctx, types.RoleProxy)

	user := setupUser(t, svcs, clock.Now().Add(time.Hour))

	sp1, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "shortcut-name",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newTestEntityDescriptor("sp1"),
			EntityID:         "sp1",
		},
	)
	require.NoError(t, err)
	require.NoError(t, svcs.spService.CreateSAMLIdPServiceProvider(ctx, sp1))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, path.Join(IdPRoute, "login", "shortcut-name"), nil)
	r = r.WithContext(authz.ContextWithUser(r.Context(), user))

	svcs.samlIdP.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	expectAuthAttemptEvent(t, svcs.emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.True(t, event.Success)
		require.Equal(t, user.Username, event.User)
		require.Empty(t, event.Error)
		require.Equal(t, "sp1", event.ServiceProviderEntityID)
	})

	node, err := html.Parse(w.Body)
	require.NoError(t, err)

	formNode := findNode(node, "form")
	require.NotNil(t, formNode)
	require.Equal(t, "method", formNode.Attr[0].Key)
	require.Equal(t, "post", formNode.Attr[0].Val)
	require.Equal(t, "action", formNode.Attr[1].Key)
	require.Equal(t, "https://sptest.iamshowcase.com/acs", formNode.Attr[1].Val)
	require.Equal(t, "id", formNode.Attr[2].Key)
	require.Equal(t, "SAMLResponseForm", formNode.Attr[2].Val)

	w = httptest.NewRecorder()
	r = httptest.NewRequest(method, path.Join(IdPRoute, "login/doesntexist"), nil)
	r = r.WithContext(authz.ContextWithUser(r.Context(), user))

	svcs.samlIdP.ServeHTTP(w, r)
	require.Equal(t, http.StatusNotFound, w.Code)

	expectAuthAttemptEvent(t, svcs.emitter, func(event *apievents.SAMLIdPAuthAttempt) {
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
	svcs := samlTestService(ctx, t, clock)

	user := setupUser(t, svcs, clock.Now().Add(time.Hour))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, path.Join(IdPRoute, "metadata"), nil)
	r = r.WithContext(authz.ContextWithUser(r.Context(), user))

	// Make sure the first access works
	svcs.samlIdP.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	lock, err := types.NewLock("test-lock", types.LockSpecV2{
		Target: types.LockTarget{
			User: user.Username,
		},
	})
	require.NoError(t, err)
	svcs.accessService.UpsertLock(ctx, lock)

	// After adding the lock, the GET should fail.
	require.Eventually(t, func() bool {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, path.Join(IdPRoute, "metadata"), nil)
		r = r.WithContext(authz.ContextWithUser(r.Context(), user))
		svcs.samlIdP.ServeHTTP(w, r)
		return w.Code == http.StatusNotFound
	}, time.Second*3, time.Millisecond*250)
}

func setupUser(t *testing.T, svcs testServices, expireTime time.Time) authz.LocalUser {
	ctx := context.Background()

	type client struct {
		services.Access
		services.Identity
	}

	clt := client{
		Access:   svcs.accessService,
		Identity: svcs.userService,
	}

	role, err := auth.CreateRole(ctx, clt, "test-group", types.RoleSpecV6{})
	require.NoError(t, err)

	user, err := types.NewUser("user1")
	user.AddRole(role.GetName())
	require.NoError(t, err)
	user, err = svcs.userService.CreateUser(ctx, user)
	require.NoError(t, err)

	identity := tlsca.Identity{
		Username: user.GetName(),
		Groups:   []string{"test-group"},
		Expires:  expireTime,
	}

	s, err := identity.Subject()
	require.NoError(t, err)
	s.Names = s.ExtraNames

	return authz.LocalUser{
		Username: user.GetName(),
		Identity: identity,
	}
}

func findNode(node *html.Node, name string) *html.Node {
	if node.Data == name {
		return node
	}

	childNode := node.FirstChild
	for childNode != nil {
		foundNode := findNode(childNode, name)
		if foundNode != nil {
			return foundNode
		}
		childNode = childNode.NextSibling
	}

	return nil
}
