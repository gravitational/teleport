package saml

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/idp/saml/testenv"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
)

func TestGetSession(t *testing.T) {
	ctx := context.Background()

	clock := clockwork.NewFakeClock()
	env := newTEnv(ctx, t, clock)

	idp, err := env.samlIdPService.createIdP(ctx)
	require.NoError(t, err)

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

	// Create testing user.
	expireTime := clock.Now().Add(time.Hour)

	// No user in the request.
	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://test-url/", nil)
	authnReq := newAuthnReq(&idp, req, sp1.GetEntityID())
	require.Nil(t, env.samlIdPService.GetSession(rw, req, authnReq))
	require.Equal(t, http.StatusForbidden, rw.Code)

	expectAuthAttemptEvent(t, env.testServices.Emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.False(t, event.Success)
		require.Empty(t, event.User)
		require.Equal(t, "access denied", event.Error)
		require.Equal(t, sp1.GetEntityID(), event.ServiceProviderEntityID)
	})

	user := setupUser(t, env.testServices, expireTime)

	// Valid user.
	rw = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "https://test-url/", bytes.NewBuffer([]byte{})).WithContext(authz.ContextWithUser(ctxWithIdentity(ctx, &user.Identity), user))
	authnReq = newAuthnReq(&idp, req, sp1.GetEntityID())
	firstSession := env.samlIdPService.GetSession(rw, req, authnReq)
	require.Equal(t, http.StatusOK, rw.Code)
	require.NotNil(t, firstSession)
	require.Equal(t, expireTime, firstSession.ExpireTime)

	result := rw.Result()
	require.NoError(t, result.Body.Close())

	// Subsequent requests should return a similar session.
	authnReq = newAuthnReq(&idp, req, sp1.GetEntityID())
	secondSession := env.samlIdPService.GetSession(rw, req, authnReq)
	require.Equal(t, http.StatusOK, rw.Code)
	require.Empty(t, cmp.Diff(firstSession, secondSession, cmpopts.IgnoreFields(saml.Session{}, "ID", "Index")))

	// User Identity is expired.
	clock.Advance(12 * time.Hour)

	rw = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "http://test-url/", bytes.NewBuffer([]byte{})).WithContext(authz.ContextWithUser(ctxWithIdentity(ctx, &user.Identity), user))
	authnReq = newAuthnReq(&idp, req, sp1.GetEntityID())
	require.Nil(t, env.samlIdPService.GetSession(rw, req, authnReq))
	require.Equal(t, http.StatusUnauthorized, rw.Code)

	expectAuthAttemptEvent(t, env.testServices.Emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.False(t, event.Success)
		require.Equal(t, user.Username, event.User)
		require.Contains(t, event.Error, "identity is expired")
		require.Equal(t, sp1.GetEntityID(), event.ServiceProviderEntityID)
	})
}

func TestGetServiceProvider(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	env := newTEnv(ctx, t, clock)
	user := setupUser(t, env.testServices, clock.Now().Add(time.Hour))
	authctx := authz.ContextWithUser(ctxWithIdentity(ctx, &user.Identity), user)

	// There are no service providers, so expect an error.
	r := httptest.NewRequest("GET", "/", bytes.NewBuffer([]byte{})).WithContext(authctx)
	newQuery := r.URL.Query()
	newQuery.Set(Webauthn.String(), "mock_value")
	r.URL.RawQuery = newQuery.Encode()

	_, err := env.samlIdPService.GetServiceProvider(r, "sp1")
	require.Error(t, err)

	expectAuthAttemptEvent(t, env.testServices.Emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.False(t, event.Success)
		require.Equal(t, user.Username, event.User)
		require.Equal(t, "could not find service provider", event.Error)
		require.Equal(t, "sp1", event.ServiceProviderEntityID)
	})

	// Add in a service provider
	sp1, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "friendly-name",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: testenv.NewTestEntityDescriptor("entity-id-1", "https://sp.com"),
			EntityID:         "entity-id-1",
		},
	)
	require.NoError(t, err)
	require.NoError(t, env.testServices.SPService.CreateSAMLIdPServiceProvider(ctx, sp1))

	// Get by friendly name fails.
	_, err = env.samlIdPService.GetServiceProvider(r, "friendly-name")
	require.Error(t, err)

	expectAuthAttemptEvent(t, env.testServices.Emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.False(t, event.Success)
		require.Equal(t, user.Username, event.User)
		require.Equal(t, "could not find service provider", event.Error)
		require.Equal(t, "friendly-name", event.ServiceProviderEntityID)
	})

	// Get by entity ID succeeds.
	ed, err := env.samlIdPService.GetServiceProvider(r, "entity-id-1")
	require.NoError(t, err)

	expectedEd, err := samlsp.ParseMetadata([]byte(sp1.GetEntityDescriptor()))
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(expectedEd, ed))

	// No event is emitted on session provider retrieval success, as it should be emitted by the subsequent
	// session provider calls.
}

func expectAuthAttemptEvent(t *testing.T, emitter *eventstest.ChannelEmitter, fn func(*apievents.SAMLIdPAuthAttempt)) {
	select {
	case event := <-emitter.C():
		authAttemptEvent, ok := event.(*apievents.SAMLIdPAuthAttempt)
		require.True(t, ok, "expected SAMLIdPAuthAttempt event, got %T", event)
		require.Equal(t, events.SAMLIdPAuthAttemptCode, authAttemptEvent.GetCode())
		fn(authAttemptEvent)
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for event")
	}
}

func newAuthnReq(idp *saml.IdentityProvider, req *http.Request, entityID string) *saml.IdpAuthnRequest {
	var ed *saml.EntityDescriptor
	if entityID != "" {
		ed = &saml.EntityDescriptor{
			EntityID: entityID,
		}
	}
	return &saml.IdpAuthnRequest{
		IDP:                     idp,
		HTTPRequest:             req,
		ServiceProviderMetadata: ed,
	}
}
