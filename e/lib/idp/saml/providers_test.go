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
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestGetSession(t *testing.T) {
	ctx := context.Background()

	clock := clockwork.NewFakeClock()
	svcs := samlTestService(ctx, t, clock)

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
	authnReq := newAuthnReq(&svcs.samlIdP.idp, req)
	require.Nil(t, svcs.samlIdP.GetSession(rw, req, authnReq))
	require.Equal(t, http.StatusForbidden, rw.Code)

	// Add in the users
	user1, err := types.NewUser("test-user")
	require.NoError(t, err)
	user2, err := types.NewUser("test-user2")
	require.NoError(t, err)

	user1.AddRole("auditor")
	user1.AddRole("editor")
	require.NoError(t, svcs.userService.CreateUser(user1))
	require.NoError(t, svcs.userService.CreateUser(user2))

	// Valid user.
	rw = httptest.NewRecorder()
	localUserCtx := context.WithValue(ctx, identityContextKey, localUserIdentity)
	req = httptest.NewRequest(http.MethodGet, "https://test-url/", bytes.NewBuffer([]byte{})).WithContext(localUserCtx)
	authnReq = newAuthnReq(&svcs.samlIdP.idp, req)
	firstSession := svcs.samlIdP.GetSession(rw, req, authnReq)

	require.Equal(t, http.StatusOK, rw.Code)
	require.NotNil(t, firstSession)
	require.Equal(t, expireTime, firstSession.ExpireTime)

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

	authnReq = newAuthnReq(&svcs.samlIdP.idp, req)
	secondSession := svcs.samlIdP.GetSession(rw, req, authnReq)
	require.Equal(t, http.StatusOK, rw.Code)
	require.Empty(t, cmp.Diff(firstSession, secondSession))

	// Get a session that doesn't belong to the user.
	rw = httptest.NewRecorder()
	mismatchedUserCtx := context.WithValue(ctx, identityContextKey, mismatchedUserIdentity)
	req = httptest.NewRequest(http.MethodGet, "http://test-url/", bytes.NewBuffer([]byte{})).WithContext(mismatchedUserCtx)
	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: firstSession.ID,
	})

	authnReq = newAuthnReq(&svcs.samlIdP.idp, req)
	mismatchedSession := svcs.samlIdP.GetSession(rw, req, authnReq)
	require.Nil(t, mismatchedSession)
	require.Equal(t, http.StatusForbidden, rw.Code)

	// No matching session.
	rw = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "http://test-url/", bytes.NewBuffer([]byte{})).WithContext(localUserCtx)
	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: "non-existent-ID",
	})
	authnReq = newAuthnReq(&svcs.samlIdP.idp, req)
	require.Nil(t, svcs.samlIdP.GetSession(rw, req, authnReq))
	require.Equal(t, http.StatusForbidden, rw.Code)

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
	authnReq = newAuthnReq(&svcs.samlIdP.idp, req)
	require.Nil(t, svcs.samlIdP.GetSession(rw, req, authnReq))
	require.Equal(t, http.StatusForbidden, rw.Code)
}

func TestGetServiceProvider(t *testing.T) {
	ctx := context.Background()

	clock := clockwork.NewFakeClock()
	svcs := samlTestService(ctx, t, clock)

	// There are no service providers, so expect an error.
	r := httptest.NewRequest("GET", "/", bytes.NewBuffer([]byte{}))
	_, err := svcs.samlIdP.GetServiceProvider(r, "sp1")
	require.Error(t, err)

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

	// Get by entity ID succeeds.
	ed, err := svcs.samlIdP.GetServiceProvider(r, "entity-id-1")
	require.NoError(t, err)

	expectedEd, err := samlsp.ParseMetadata([]byte(sp1.GetEntityDescriptor()))
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(expectedEd, ed))
}

func newAuthnReq(idp *saml.IdentityProvider, req *http.Request) *saml.IdpAuthnRequest {
	return &saml.IdpAuthnRequest{
		IDP:         idp,
		HTTPRequest: req,
	}
}
