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
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestMetadata(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	svcs := samlTestService(ctx, t, clock)

	user := setupUser(t, svcs, clock.Now().Add(time.Hour))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, path.Join(IdPRoute, "metadata"), nil)
	r = r.WithContext(context.WithValue(r.Context(), auth.ContextUser, user))

	svcs.samlIdP.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	ed := saml.EntityDescriptor{}
	require.NoError(t, xml.Unmarshal(w.Body.Bytes(), &ed))
	require.Equal(t, fmt.Sprintf("https://test.url%s", path.Join(IdPRoute, "metadata")), ed.EntityID)
}

func TestSSOGET(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewRealClock()
	svcs := samlTestService(ctx, t, clock)

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
	var buf bytes.Buffer
	require.NoError(t, xml.NewEncoder(&buf).Encode(authnRequest))

	var compressedBuf bytes.Buffer
	flateWriter, err := flate.NewWriter(&compressedBuf, flate.DefaultCompression)
	flateWriter.Write(buf.Bytes())
	require.NoError(t, flateWriter.Close())

	encodedRequest := base64.StdEncoding.EncodeToString(compressedBuf.Bytes())
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", path.Join(IdPRoute, "sso"), nil)
	r = r.WithContext(context.WithValue(r.Context(), auth.ContextUser, user))

	values := r.URL.Query()
	values.Add("SAMLRequest", encodedRequest)
	r.URL.RawQuery = values.Encode()

	svcs.samlIdP.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
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

func TestSSOPOST(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewRealClock()
	svcs := samlTestService(ctx, t, clock)

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
	var buf bytes.Buffer
	require.NoError(t, xml.NewEncoder(&buf).Encode(authnRequest))

	encodedRequest := base64.StdEncoding.EncodeToString(buf.Bytes())
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, path.Join(IdPRoute, "sso"), nil)
	r = r.WithContext(context.WithValue(r.Context(), auth.ContextUser, user))

	r.PostForm = url.Values{}
	r.PostForm.Add("SAMLRequest", encodedRequest)

	svcs.samlIdP.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
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

func TestIDPInitiatedLoginGET(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewRealClock()
	svcs := samlTestService(ctx, t, clock)

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
	r := httptest.NewRequest(http.MethodGet, path.Join(IdPRoute, "login", "shortcut-name"), nil)
	r = r.WithContext(context.WithValue(r.Context(), auth.ContextUser, user))

	svcs.samlIdP.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)

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

func TestIDPInitiatedLoginPOST(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewRealClock()
	svcs := samlTestService(ctx, t, clock)

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
	r := httptest.NewRequest(http.MethodPost, path.Join(IdPRoute, "login", "shortcut-name"), nil)
	r = r.WithContext(context.WithValue(r.Context(), auth.ContextUser, user))

	svcs.samlIdP.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)

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

func TestLockUser(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	svcs := samlTestService(ctx, t, clock)

	user := setupUser(t, svcs, clock.Now().Add(time.Hour))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, path.Join(IdPRoute, "metadata"), nil)
	r = r.WithContext(context.WithValue(r.Context(), auth.ContextUser, user))

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
		r = r.WithContext(context.WithValue(r.Context(), auth.ContextUser, user))
		svcs.samlIdP.ServeHTTP(w, r)
		return w.Code == http.StatusNotFound
	}, time.Second*3, time.Millisecond*250)
}

func setupUser(t *testing.T, svcs testServices, expireTime time.Time) auth.LocalUser {
	role, err := types.NewRole("test-group", types.RoleSpecV6{})
	require.NoError(t, err)
	require.NoError(t, svcs.accessService.CreateRole(context.Background(), role))

	user, err := types.NewUser("user1")
	user.AddRole(role.GetName())
	require.NoError(t, err)
	require.NoError(t, svcs.userService.CreateUser(user))

	identity := tlsca.Identity{
		Username: user.GetName(),
		Groups:   []string{"test-group"},
		Expires:  expireTime,
	}

	s, err := identity.Subject()
	require.NoError(t, err)
	s.Names = s.ExtraNames

	return auth.LocalUser{
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
