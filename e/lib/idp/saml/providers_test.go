/*
Copyright 2023 Gravitational, Inc.

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
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestGetSession(t *testing.T) {
	ctx := context.Background()

	clock := clockwork.NewFakeClock()
	svcs := samlTestService(ctx, t, clock)

	idp, err := svcs.samlIdP.createIdP(ctx)
	require.NoError(t, err)

	// Create testing user.
	expireTime := clock.Now().Add(time.Hour)
	localUserIdentity := &tlsca.Identity{
		Username: "test-user",
		Expires:  expireTime,
	}

	// Create an user that doesn't own any sessions.
	mismatchedUserIdentity := &tlsca.Identity{
		Username: "test-user2",
		Expires:  expireTime,
	}

	// No user in the request.
	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://test-url/", nil)
	authnReq := newAuthnReq(&idp, req, "")
	require.Nil(t, svcs.samlIdP.GetSession(rw, req, authnReq))
	require.Equal(t, http.StatusForbidden, rw.Code)

	expectAuthAttemptEvent(t, svcs.emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.False(t, event.Success)
		require.Empty(t, event.User)
		require.Equal(t, "access denied", event.Error)
		require.Empty(t, event.ServiceProviderEntityID)
	})

	// Add in the users
	user1, err := types.NewUser("test-user")
	require.NoError(t, err)
	user2, err := types.NewUser("test-user2")
	require.NoError(t, err)

	user1.AddRole("auditor")
	user1.AddRole("editor")
	_, err = svcs.userService.CreateUser(ctx, user1)
	require.NoError(t, err)
	_, err = svcs.userService.CreateUser(ctx, user2)
	require.NoError(t, err)

	// Valid user.
	entityID := "entity-id"
	rw = httptest.NewRecorder()
	localUserCtx := context.WithValue(ctx, identityContextKey, localUserIdentity)
	req = httptest.NewRequest(http.MethodGet, "https://test-url/", bytes.NewBuffer([]byte{})).WithContext(localUserCtx)
	authnReq = newAuthnReq(&idp, req, entityID)
	firstSession := svcs.samlIdP.GetSession(rw, req, authnReq)

	require.Equal(t, http.StatusOK, rw.Code)
	require.NotNil(t, firstSession)
	require.Equal(t, expireTime, firstSession.ExpireTime)

	expectAuthAttemptEvent(t, svcs.emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.True(t, event.Success)
		require.Equal(t, localUserIdentity.Username, event.User)
		require.Empty(t, event.Error)
		require.Equal(t, entityID, event.ServiceProviderEntityID)
	})

	result := rw.Result()
	require.NoError(t, result.Body.Close())
	require.Len(t, result.Cookies(), 1)
	require.Empty(t, cmp.Diff(&http.Cookie{
		Name:     sessionCookieName,
		Value:    firstSession.ID,
		MaxAge:   3600, // This should be 1 hour in seconds.
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
	}, result.Cookies()[0], cmpopts.IgnoreFields(http.Cookie{}, "Raw")))

	// Get the same session.
	rw = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "http://test-url/", bytes.NewBuffer([]byte{})).WithContext(localUserCtx)
	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: firstSession.ID,
	})

	authnReq = newAuthnReq(&idp, req, entityID)
	secondSession := svcs.samlIdP.GetSession(rw, req, authnReq)
	require.Equal(t, http.StatusOK, rw.Code)
	require.Empty(t, cmp.Diff(firstSession, secondSession))

	expectAuthAttemptEvent(t, svcs.emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.True(t, event.Success)
		require.Equal(t, localUserIdentity.Username, event.User)
		require.Empty(t, event.Error)
		require.Equal(t, entityID, event.ServiceProviderEntityID)
	})

	// Get a session that doesn't belong to the user.
	rw = httptest.NewRecorder()
	mismatchedUserCtx := context.WithValue(ctx, identityContextKey, mismatchedUserIdentity)
	req = httptest.NewRequest(http.MethodGet, "http://test-url/", bytes.NewBuffer([]byte{})).WithContext(mismatchedUserCtx)
	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: firstSession.ID,
	})

	authnReq = newAuthnReq(&idp, req, entityID)
	mismatchedSession := svcs.samlIdP.GetSession(rw, req, authnReq)
	require.Nil(t, mismatchedSession)
	require.Equal(t, http.StatusForbidden, rw.Code)

	expectAuthAttemptEvent(t, svcs.emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.False(t, event.Success)
		require.Equal(t, mismatchedUserIdentity.Username, event.User)
		require.Equal(t, "user test-user2 attempted to access a SAML IdP session that belonged to test-user", event.Error)
		require.Equal(t, entityID, event.ServiceProviderEntityID)
	})

	// No matching session.
	rw = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "http://test-url/", bytes.NewBuffer([]byte{})).WithContext(localUserCtx)
	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: "non-existent-ID",
	})
	authnReq = newAuthnReq(&idp, req, entityID)
	require.Nil(t, svcs.samlIdP.GetSession(rw, req, authnReq))
	require.Equal(t, http.StatusForbidden, rw.Code)

	expectAuthAttemptEvent(t, svcs.emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.False(t, event.Success)
		require.Equal(t, localUserIdentity.Username, event.User)
		require.Equal(t, "user test-user attempted to access a non-existent SAML IdP session", event.Error)
		require.Equal(t, entityID, event.ServiceProviderEntityID)
	})

	// Session is expired.
	clock.Advance(12 * time.Hour)
	localUserIdentity.Expires = clock.Now().Add(time.Hour)

	rw = httptest.NewRecorder()
	localUserCtx = context.WithValue(ctx, identityContextKey, localUserIdentity)
	req = httptest.NewRequest(http.MethodGet, "http://test-url/", bytes.NewBuffer([]byte{})).WithContext(localUserCtx)
	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: firstSession.ID,
	})
	authnReq = newAuthnReq(&idp, req, entityID)
	require.Nil(t, svcs.samlIdP.GetSession(rw, req, authnReq))
	require.Equal(t, http.StatusForbidden, rw.Code)

	expectAuthAttemptEvent(t, svcs.emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.False(t, event.Success)
		require.Equal(t, localUserIdentity.Username, event.User)
		require.Equal(t, "user test-user attempted to access a non-existent SAML IdP session", event.Error)
		require.Equal(t, entityID, event.ServiceProviderEntityID)
	})
}

func TestGetServiceProvider(t *testing.T) {
	ctx := context.Background()

	clock := clockwork.NewFakeClock()
	svcs := samlTestService(ctx, t, clock)

	// Create testing user.
	localUserIdentity := &tlsca.Identity{
		Username: "test-user",
		Expires:  clock.Now().Add(time.Hour),
	}

	// There are no service providers, so expect an error.
	localUserCtx := context.WithValue(ctx, identityContextKey, localUserIdentity)
	r := httptest.NewRequest("GET", "/", bytes.NewBuffer([]byte{})).WithContext(localUserCtx)
	_, err := svcs.samlIdP.GetServiceProvider(r, "sp1")
	require.Error(t, err)

	expectAuthAttemptEvent(t, svcs.emitter, func(event *apievents.SAMLIdPAuthAttempt) {
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
			EntityDescriptor: newTestEntityDescriptor("entity-id-1"),
			EntityID:         "entity-id-1",
		},
	)
	require.NoError(t, err)
	require.NoError(t, svcs.spService.CreateSAMLIdPServiceProvider(ctx, sp1))

	// Get by friendly name fails.
	_, err = svcs.samlIdP.GetServiceProvider(r, "friendly-name")
	require.Error(t, err)

	expectAuthAttemptEvent(t, svcs.emitter, func(event *apievents.SAMLIdPAuthAttempt) {
		require.False(t, event.Success)
		require.Equal(t, localUserIdentity.Username, event.User)
		require.Equal(t, "could not find service provider", event.Error)
		require.Equal(t, "friendly-name", event.ServiceProviderEntityID)
	})

	// Get by entity ID succeeds.
	ed, err := svcs.samlIdP.GetServiceProvider(r, "entity-id-1")
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
