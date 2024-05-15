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
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestGetSession(t *testing.T) {
	ctx := context.Background()

	clock := clockwork.NewFakeClock()
	env := newTEnv(ctx, t, clock)

	idp, err := env.samlIdPService.createIdP(ctx)
	require.NoError(t, err)

	// Create testing user.
	expireTime := clock.Now().Add(time.Hour)
	localUserIdentity := &tlsca.Identity{
		Username: "test-user",
		Expires:  expireTime,
	}

	// No user in the request.
	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://test-url/", nil)
	authnReq := newAuthnReq(&idp, req, "")
	require.Nil(t, env.samlIdPService.GetSession(rw, req, authnReq))
	require.Equal(t, http.StatusForbidden, rw.Code)

	expectAuthAttemptEvent(t, env.testServices.Emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.False(t, event.Success)
		require.Empty(t, event.User)
		require.Equal(t, "access denied", event.Error)
		require.Empty(t, event.ServiceProviderEntityID)
	})

	// Add in the users
	user1, err := types.NewUser("test-user")
	require.NoError(t, err)

	user1.AddRole("auditor")
	user1.AddRole("editor")
	_, err = env.testServices.UserService.CreateUser(ctx, user1)
	require.NoError(t, err)

	// Valid user.
	entityID := "entity-id"
	rw = httptest.NewRecorder()
	localUserCtx := context.WithValue(ctx, identityContextKey, localUserIdentity)
	req = httptest.NewRequest(http.MethodGet, "https://test-url/", bytes.NewBuffer([]byte{})).WithContext(localUserCtx)
	authnReq = newAuthnReq(&idp, req, entityID)
	firstSession := env.samlIdPService.GetSession(rw, req, authnReq)

	require.Equal(t, http.StatusOK, rw.Code)
	require.NotNil(t, firstSession)
	require.Equal(t, expireTime, firstSession.ExpireTime)

	expectAuthAttemptEvent(t, env.testServices.Emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.True(t, event.Success)
		require.Equal(t, localUserIdentity.Username, event.User)
		require.Empty(t, event.Error)
		require.Equal(t, entityID, event.ServiceProviderEntityID)
	})

	result := rw.Result()
	require.NoError(t, result.Body.Close())

	// Subsequent requests should return a similar session.
	authnReq = newAuthnReq(&idp, req, entityID)
	secondSession := env.samlIdPService.GetSession(rw, req, authnReq)
	require.Equal(t, http.StatusOK, rw.Code)
	require.Empty(t, cmp.Diff(firstSession, secondSession, cmpopts.IgnoreFields(saml.Session{}, "ID", "Index")))

	expectAuthAttemptEvent(t, env.testServices.Emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.True(t, event.Success)
		require.Equal(t, localUserIdentity.Username, event.User)
		require.Empty(t, event.Error)
		require.Equal(t, entityID, event.ServiceProviderEntityID)
	})

	// User Identity is expired.
	clock.Advance(12 * time.Hour)

	rw = httptest.NewRecorder()
	localUserCtx = context.WithValue(ctx, identityContextKey, localUserIdentity)
	req = httptest.NewRequest(http.MethodGet, "http://test-url/", bytes.NewBuffer([]byte{})).WithContext(localUserCtx)

	authnReq = newAuthnReq(&idp, req, entityID)
	require.Nil(t, env.samlIdPService.GetSession(rw, req, authnReq))
	require.Equal(t, http.StatusForbidden, rw.Code)

	expectAuthAttemptEvent(t, env.testServices.Emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.False(t, event.Success)
		require.Equal(t, localUserIdentity.Username, event.User)
		require.Equal(t, "access denied", event.Error)
		require.Equal(t, entityID, event.ServiceProviderEntityID)
	})
}

func TestGetServiceProvider(t *testing.T) {
	ctx := context.Background()

	clock := clockwork.NewFakeClock()
	env := newTEnv(ctx, t, clock)

	// Create testing user.
	localUserIdentity := &tlsca.Identity{
		Username: "test-user",
		Expires:  clock.Now().Add(time.Hour),
	}

	// There are no service providers, so expect an error.
	localUserCtx := context.WithValue(ctx, identityContextKey, localUserIdentity)
	r := httptest.NewRequest("GET", "/", bytes.NewBuffer([]byte{})).WithContext(localUserCtx)
	_, err := env.samlIdPService.GetServiceProvider(r, "sp1")
	require.Error(t, err)

	expectAuthAttemptEvent(t, env.testServices.Emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.False(t, event.Success)
		require.Equal(t, localUserIdentity.Username, event.User)
		require.Equal(t, "could not find service provider", event.Error)
		require.Equal(t, "sp1", event.ServiceProviderEntityID)
	})

	// Add in a service provider
	sp1, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "friendly-name",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: testenv.NewTestEntityDescriptor("entity-id-1"),
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
		require.Equal(t, localUserIdentity.Username, event.User)
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
