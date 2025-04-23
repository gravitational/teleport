/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package img

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/distribution/reference"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/uuid"
	ociremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var distrolessKey = []byte("-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEWZzVzkb8A+DbgDpaJId/bOmV8n7Q\nOqxYbK0Iro6GzSmOzxkn+N2AKawLyXi84WSwJQBK//psATakCgAQKkNTAA==\n-----END PUBLIC KEY-----")

func Test_NewCosignSignleKeyValidator(t *testing.T) {
	a, err := NewCosignSingleKeyValidator(distrolessKey, "distroless", nil)
	require.NoError(t, err)
	require.Equal(t, "distroless-799a5c21a7f8c39707274cbd065ba2e1969d8d29", a.Name())
}

// Test_cosignKeyValidator_ValidateAndResolveDigest tests both the resolution and the validation
// of images in a public registry.
func Test_cosignKeyValidator_ValidateAndResolveDigest(t *testing.T) {
	// Setup and start a test OCI registry

	// Referrer API is enabled even though the signature manifests don't have the `Subject` field set.
	// This is the worst case scenario and also reduces the amount of noise and failed calls in the logs.
	testRegistry := httptest.NewServer(registry.New(registry.WithReferrersSupport(true)))
	t.Cleanup(testRegistry.Close)

	// Put test layers and manifests into the registry
	for digest, contents := range blobs {
		uploadBlob(t, testRegistry, digest, contents, "")
	}
	for manifest, contents := range manifests {
		uploadManifest(t, testRegistry, manifest, contents, "")
	}
	regURL, err := url.Parse(testRegistry.URL)
	require.NoError(t, err)

	// Build a special test case: an image with a tag that must be resolved
	namedImage, err := reference.ParseNamed(regURL.Host + "/testrepo")
	require.NoError(t, err)
	signedManifestTaggedImage, err := reference.WithTag(namedImage, "signed-manifest")
	require.NoError(t, err)
	uploadManifest(t, testRegistry, "signed-manifest", manifests[signedManifest], "")

	// Build a validator
	pubKey, err := cryptoutils.UnmarshalPEMToPublicKey(publicKey)
	require.NoError(t, err)
	skid, err := cryptoutils.SKID(pubKey)
	require.NoError(t, err)
	verifier, err := signature.LoadVerifier(pubKey, hashAlgo)
	require.NoError(t, err)
	validator := &cosignKeyValidator{
		verifier:        verifier,
		skid:            skid,
		name:            "test",
		registryOptions: []ociremote.Option{ociremote.WithRemoteOptions(remote.WithTransport(testRegistry.Client().Transport))},
	}

	// Doing the real test: submitting several images to the validator and checking its output
	tests := []struct {
		name      string
		image     reference.NamedTagged
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "signed manifest",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", signedManifest),
			assertErr: require.NoError,
		},
		{
			name:      "signed manifest needs resolution",
			image:     signedManifestTaggedImage,
			assertErr: require.NoError,
		},
		{
			name:      "unsigned manifest",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", unsignedManifest),
			assertErr: require.Error,
		},
		{
			name:      "untrusted signed manifest",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", untrustedSignedManifest),
			assertErr: require.Error,
		},
		{
			name:      "double signed manifest",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", doubleSignedManifest),
			assertErr: require.NoError,
		},
		{
			name:      "untrusted double signed manifest",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", untrustedDoubleSignedManifest),
			assertErr: require.Error,
		},
		{
			name:      "wrongly signed manifest",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", wronglySignedManifest),
			assertErr: require.Error,
		},
		{
			name:      "untrusted wrongly signed manifest",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", untrustedWronglySignedManifest),
			assertErr: require.Error,
		},
		{
			name:      "signed index",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", signedIndex),
			assertErr: require.NoError,
		},
		{
			name:      "unsigned index",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", unsignedIndex),
			assertErr: require.Error,
		},
		{
			name:      "untrusted signed index",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", untrustedSignedIndex),
			assertErr: require.Error,
		},
		{
			name:      "double signed index",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", doubleSignedIndex),
			assertErr: require.NoError,
		},
		{
			name:      "untrusted double signed index",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", untrustedDoubleSignedIndex),
			assertErr: require.Error,
		},
		{
			name:      "wrongly signed index",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", wronglySignedIndex),
			assertErr: require.Error,
		},
		{
			name:      "untrusted wrongly signed index",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", untrustdedWronglySignedIndex),
			assertErr: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validator.ValidateAndResolveDigest(context.Background(), tt.image)
			tt.assertErr(t, err)
		})
	}
}

func uploadBlob(t *testing.T, testRegistry *httptest.Server, digest, content, authHeader string) {
	u, err := url.Parse(testRegistry.URL + "/v2/testrepo/blobs/uploads/1?digest=" + digest)
	require.NoError(t, err)
	uploadToRegistry(t, testRegistry, u, content, authHeader)
}

func uploadManifest(t *testing.T, testRegistry *httptest.Server, ref, content, authHeader string) {
	u, err := url.Parse(testRegistry.URL + "/v2/testrepo/manifests/" + ref)
	require.NoError(t, err)
	uploadToRegistry(t, testRegistry, u, content, authHeader)
}

func uploadToRegistry(t *testing.T, testRegistry *httptest.Server, u *url.URL, content, authHeader string) {
	header := http.Header{}
	if authHeader != "" {
		header.Set("Authorization", authHeader)
	}
	req := &http.Request{
		Method: "PUT",
		URL:    u,
		Header: header,
		Body:   io.NopCloser(strings.NewReader(content)),
	}
	resp, err := testRegistry.Client().Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}

// registryAuthMiddleware mocks the docker registry authentication and allows us to run cosign tests against an
// authenticated in-memory registry.
// The auth dance is described in https://github.com/google/go-containerregistry/blob/main/pkg/authn/README.md
// This dummy registry authentication accepts a single user/password pair, and a single static Bearer token.
type registryAuthMiddleware struct {
	t *testing.T

	registryHandler http.Handler
	user            string
	password        string

	// fields below are randomly generated
	service     string
	bearerToken string
}

// ServeHTTP implements http.Handler. The handler intercepts every request and checks if they are authenticated.
// Unauthenticated requests receive a 401 and are redirected to "/token".
// "/token" checks the baic auth credentials and returns the registry token.
func (auth registryAuthMiddleware) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	// Catch login requests
	if req.URL.Path == "/token" {
		// We don't check service and scope, we just see if user and pass are sent
		user, password, ok := req.BasicAuth()
		if !ok || user != auth.user || password != auth.password {
			resp.WriteHeader(http.StatusUnauthorized)
			return
		}

		resp.Header().Add("Content-Type", "application/json")
		resp.WriteHeader(http.StatusOK)
		token := struct {
			Token string `json:"token"`
		}{
			Token: auth.bearerToken,
		}
		jsonToken, err := json.Marshal(token)
		assert.NoError(auth.t, err)
		_, err = resp.Write(jsonToken)
		assert.NoError(auth.t, err)
		return
	}

	// Allow authenticated requests to pass
	if bearer := req.Header.Get("Authorization"); bearer != "" {
		assert.Equal(auth.t, auth.authorizationHeader(), bearer)
		auth.registryHandler.ServeHTTP(resp, req)
		return
	}

	// Request is not authenticated, we return 401 and redirect to the login endpoint
	realm := "http://" + req.Host + "/token"
	resp.Header().Set("Content-Type", "application/json")
	resp.Header().Set("Www-Authenticate", fmt.Sprintf("Bearer realm=%q,service=%q", realm, auth.service))
	resp.WriteHeader(http.StatusUnauthorized)
}

// authorizationHeader returns the content of the Authorization header to authenticate a request.
func (auth registryAuthMiddleware) authorizationHeader() string {
	return fmt.Sprintf("Bearer %s", auth.bearerToken)
}

// newRegistryAuthMiddleware takes the http.Handler of a docker registry and wraps it to provide
// basic auth for the registry.
func newRegistryAuthMiddleware(t *testing.T, user, password string, registryHandler http.Handler) registryAuthMiddleware {
	service := uuid.New().String()
	bearerToken := base64.StdEncoding.EncodeToString([]byte(uuid.New().String()))
	return registryAuthMiddleware{
		t:               t,
		registryHandler: registryHandler,
		user:            user,
		password:        password,
		service:         service,
		bearerToken:     bearerToken,
	}
}

// Test_cosignKeyValidator_ValidateAndResolveDigest tests both the resolution and the validation
// of images in a private registry (basic auth).
func Test_cosignKeyValidator_ValidateAndResolveDigestAuthenticated(t *testing.T) {
	// Setup and start a test OCI registry
	testUser := "user"
	testPassword := "password"

	// Referrer API is enabled even though the signature manifests don't have the `Subject` field set.
	// This is the worst case scenario and also reduces the amount of noise and failed calls in the logs.
	registryHandler := registry.New(registry.WithReferrersSupport(true))
	authRegistryHandler := newRegistryAuthMiddleware(t, testUser, testPassword, registryHandler)
	testRegistry := httptest.NewServer(authRegistryHandler)
	t.Cleanup(testRegistry.Close)
	authHeader := authRegistryHandler.authorizationHeader()

	// Put test layers and manifests into the registry
	for digest, contents := range blobs {
		uploadBlob(t, testRegistry, digest, contents, authHeader)
	}
	for manifest, contents := range manifests {
		uploadManifest(t, testRegistry, manifest, contents, authHeader)
	}

	regURL, err := url.Parse(testRegistry.URL)
	require.NoError(t, err)

	// Build a special test case: an image with a tag that must be resolved
	namedImage, err := reference.ParseNamed(regURL.Host + "/testrepo")
	require.NoError(t, err)
	signedManifestTaggedImage, err := reference.WithTag(namedImage, "signed-manifest")
	require.NoError(t, err)
	uploadManifest(t, testRegistry, "signed-manifest", manifests[signedManifest], authHeader)

	// Build a validator
	pubKey, err := cryptoutils.UnmarshalPEMToPublicKey(publicKey)
	require.NoError(t, err)
	skid, err := cryptoutils.SKID(pubKey)
	require.NoError(t, err)
	verifier, err := signature.LoadVerifier(pubKey, hashAlgo)
	require.NoError(t, err)
	basicAuth := &authn.Basic{
		Username: testUser,
		Password: testPassword,
	}

	validator := &cosignKeyValidator{
		verifier: verifier,
		skid:     skid,
		name:     "test",
		registryOptions: []ociremote.Option{
			ociremote.WithRemoteOptions(
				remote.WithTransport(testRegistry.Client().Transport),
				remote.WithAuth(basicAuth),
			),
		},
	}
	// Doing the real test: submitting several images to the validator and checking its output
	tests := []struct {
		name      string
		image     reference.NamedTagged
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "signed manifest",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", signedManifest),
			assertErr: require.NoError,
		},
		{
			name:      "signed manifest needs resolution",
			image:     signedManifestTaggedImage,
			assertErr: require.NoError,
		},
		{
			name:      "unsigned manifest",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", unsignedManifest),
			assertErr: require.Error,
		},
		{
			name:      "untrusted signed manifest",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", untrustedSignedManifest),
			assertErr: require.Error,
		},
		{
			name:      "double signed manifest",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", doubleSignedManifest),
			assertErr: require.NoError,
		},
		{
			name:      "untrusted double signed manifest",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", untrustedDoubleSignedManifest),
			assertErr: require.Error,
		},
		{
			name:      "wrongly signed manifest",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", wronglySignedManifest),
			assertErr: require.Error,
		},
		{
			name:      "untrusted wrongly signed manifest",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", untrustedWronglySignedManifest),
			assertErr: require.Error,
		},
		{
			name:      "signed index",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", signedIndex),
			assertErr: require.NoError,
		},
		{
			name:      "unsigned index",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", unsignedIndex),
			assertErr: require.Error,
		},
		{
			name:      "untrusted signed index",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", untrustedSignedIndex),
			assertErr: require.Error,
		},
		{
			name:      "double signed index",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", doubleSignedIndex),
			assertErr: require.NoError,
		},
		{
			name:      "untrusted double signed index",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", untrustedDoubleSignedIndex),
			assertErr: require.Error,
		},
		{
			name:      "wrongly signed index",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", wronglySignedIndex),
			assertErr: require.Error,
		},
		{
			name:      "untrusted wrongly signed index",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", untrustdedWronglySignedIndex),
			assertErr: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validator.ValidateAndResolveDigest(context.Background(), tt.image)
			tt.assertErr(t, err)
		})
	}
}
