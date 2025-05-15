/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package sigstore_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	bundlepb "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	commonpb "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest/sigstore"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest/sigstore/sigstoretest"
	"github.com/gravitational/teleport/lib/utils"
)

var loopbackPrefixes = []string{"127.0.0.1/8", "::1/128"}

func TestDiscovery_SimpleSigning(t *testing.T) {
	registry := sigstoretest.RunTestRegistry(t, sigstoretest.NoAuth)

	payloads, err := sigstore.Discover(
		context.Background(),
		fmt.Sprintf("%s/simple-signing:v1", registry),
		"sha256:21c76c650023cac8d753af4cb591e6f7450c6e2b499b5751d4a21e26e2fc5012",
		sigstore.DiscoveryConfig{
			Logger:                        utils.NewSlogLoggerForTests(),
			AllowedPrivateNetworkPrefixes: loopbackPrefixes,
		},
	)
	require.NoError(t, err)
	require.Len(t, payloads, 2)

	t.Run("keyless signature", func(t *testing.T) {
		var bundle bundlepb.Bundle
		err = proto.Unmarshal(payloads[0].Bundle, &bundle)
		require.NoError(t, err)

		assert.Equal(t, "application/vnd.dev.sigstore.bundle+json;version=0.1", bundle.MediaType)

		// Certificate chain.
		vm := bundle.GetVerificationMaterial()
		assert.Len(t, vm.GetX509CertificateChain().GetCertificates(), 3)

		// Transparency log entry.
		require.Len(t, vm.GetTlogEntries(), 1)
		entry := vm.GetTlogEntries()[0]
		assert.NotNil(t, entry.GetLogId().GetKeyId())
		assert.Equal(t, int64(179659750), entry.LogIndex)
		assert.Equal(t, int64(1741602770), entry.IntegratedTime)
		assert.Equal(t,
			"3045022037e014f2167b95a2d3318e60adab1dce59d213db382e7175c366549e978480ef022100921c6c4d5825c3d96962af5cf350cef1e4f0a38a4f59205df20d0bfac0d23d68",
			hex.EncodeToString(entry.GetInclusionPromise().GetSignedEntryTimestamp()),
		)
		assert.Equal(t, "hashedrekord", entry.GetKindVersion().GetKind())
		assert.Equal(t, "0.0.1", entry.GetKindVersion().GetVersion())
		assert.NotNil(t, entry.GetCanonicalizedBody())

		// Signature.
		signature := bundle.GetMessageSignature()
		assert.Equal(t,
			"3046022100f0689e78084f401c660ed8e1c5ff2410bd1ea08e4636b5a645e3742c9ad57d3f022100f2330c7ac07cbb835387285a3405a0190b4ee9aa301a6e9c2ff68454f032330f",
			hex.EncodeToString(signature.GetSignature()),
		)
		assert.Equal(t,
			commonpb.HashAlgorithm_SHA2_256.String(),
			signature.GetMessageDigest().GetAlgorithm().String(),
		)
		assert.Equal(t,
			"608c32cf47678ef8e63578597fbb982ddf3b84cb491359e7c6e824b3925a8ce2",
			hex.EncodeToString(signature.GetMessageDigest().GetDigest()),
		)

		// Simple signing envelope.
		var envelope struct {
			Critical struct {
				Image struct {
					Digest string `json:"docker-manifest-digest"`
				} `json:"image"`
			} `json:"critical"`
		}
		err = json.Unmarshal(payloads[0].SimpleSigningEnvelope, &envelope)
		require.NoError(t, err)
		assert.Equal(t, "sha256:21c76c650023cac8d753af4cb591e6f7450c6e2b499b5751d4a21e26e2fc5012", envelope.Critical.Image.Digest)
	})

	t.Run("key-based signature", func(t *testing.T) {
		var bundle bundlepb.Bundle
		err = proto.Unmarshal(payloads[1].Bundle, &bundle)
		require.NoError(t, err)

		// Public key.
		key := bundle.GetVerificationMaterial().GetPublicKey()
		require.NotNil(t, key)
	})
}

func TestDiscovery_Attestations(t *testing.T) {
	registry := sigstoretest.RunTestRegistry(t, sigstoretest.NoAuth)

	payloads, err := sigstore.Discover(
		context.Background(),
		fmt.Sprintf("%s/attestations:v1", registry),
		"sha256:32c91fcdf8b41ef78cf63e7be080a366597fd5a748480f5d2a6dc0cff5203807",
		sigstore.DiscoveryConfig{
			Logger:                        utils.NewSlogLoggerForTests(),
			AllowedPrivateNetworkPrefixes: loopbackPrefixes,
		},
	)
	require.NoError(t, err)
	require.Len(t, payloads, 1)

	var bundle bundlepb.Bundle
	err = proto.Unmarshal(payloads[0].Bundle, &bundle)
	require.NoError(t, err)

	var attestation struct {
		Type          string `json:"_type"`
		PredicateType string `json:"predicateType"`
	}
	err = json.Unmarshal(bundle.GetDsseEnvelope().GetPayload(), &attestation)
	require.NoError(t, err)
	assert.Equal(t, "https://in-toto.io/Statement/v1", attestation.Type)
	assert.Equal(t, "https://slsa.dev/provenance/v1", attestation.PredicateType)
}

func TestDiscovery_InfiniteRedirects(t *testing.T) {
	// Check that a malicious registry can't DoS us with infinite redirects.
	var requests int
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		}),
	)
	t.Cleanup(server.Close)

	regURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	_, err = sigstore.Discover(
		context.Background(),
		fmt.Sprintf("%s/foo:v1", regURL.Host),
		"sha256:32c91fcdf8b41ef78cf63e7be080a366597fd5a748480f5d2a6dc0cff5203807",
		sigstore.DiscoveryConfig{
			Logger:                        utils.NewSlogLoggerForTests(),
			AllowedPrivateNetworkPrefixes: loopbackPrefixes,
		},
	)
	require.NoError(t, err)
	require.Equal(t, 10, requests)
}
