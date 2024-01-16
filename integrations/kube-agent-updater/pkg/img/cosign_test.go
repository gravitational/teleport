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
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	ociremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/stretchr/testify/require"
)

var distrolessKey = []byte("-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEWZzVzkb8A+DbgDpaJId/bOmV8n7Q\nOqxYbK0Iro6GzSmOzxkn+N2AKawLyXi84WSwJQBK//psATakCgAQKkNTAA==\n-----END PUBLIC KEY-----")

func Test_NewCosignSignleKeyValidator(t *testing.T) {
	a, err := NewCosignSingleKeyValidator(distrolessKey, "distroless", nil)
	require.NoError(t, err)
	require.Equal(t, "distroless-799a5c21a7f8c39707274cbd065ba2e1969d8d29", a.Name())
}

// We don't test the digest resolution here (we call the validation function with
// a digested reference, the resolution step will return the digest instead of
// contacting the upstream to get it.
func Test_cosignKeyValidator_ValidateAndResolveDigest(t *testing.T) {
	// Setup and start a test OCI registry

	// Referrer API is enabled even though the signature manifests don't have the `Subject` field set.
	// This is the worst case scenario and also reduces the amount of noise and failed calls in the logs.
	testRegistry := httptest.NewServer(registry.New(registry.WithReferrersSupport(true)))
	t.Cleanup(testRegistry.Close)

	// Put test layers and manifests into the registry
	for digest, contents := range blobs {
		u, err := url.Parse(testRegistry.URL + "/v2/testrepo/blobs/uploads/1?digest=" + digest)
		require.NoError(t, err)
		req := &http.Request{
			Method: "PUT",
			URL:    u,
			Body:   io.NopCloser(strings.NewReader(contents)),
		}
		resp, err := testRegistry.Client().Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
	}
	for manifest, contents := range manifests {
		u, err := url.Parse(testRegistry.URL + "/v2/testrepo/manifests/" + manifest)
		require.NoError(t, err)
		req := &http.Request{
			Method: "PUT",
			URL:    u,
			Body:   io.NopCloser(strings.NewReader(contents)),
		}
		resp, err := testRegistry.Client().Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
	}

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

	regURL, err := url.Parse(testRegistry.URL)
	require.NoError(t, err)

	// Doing the real test: submitting several images to the validator and checking its output
	tests := []struct {
		name      string
		image     NamedTaggedDigested
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "signed manifest",
			image:     NewImageRef(regURL.Host, "testrepo", "not-resolved", signedManifest),
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
