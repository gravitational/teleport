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
package sigstore

import (
	"context"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/gravitational/trace"
	bundlepb "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

const mediaTypePrefixBundle = "application/vnd.dev.sigstore.bundle"

// BundleDiscoveryMethod discovers attestation bundles linked to an image using the
// OCI Referrers API, such as GitHub build provenance.
type BundleDiscoveryMethod struct {
	repo   *Repository
	digest v1.Hash
}

// NewBundleDiscoveryMethod creates a BundleDiscoveryMethod.
func NewBundleDiscoveryMethod(repo *Repository, digest v1.Hash) *BundleDiscoveryMethod {
	return &BundleDiscoveryMethod{repo, digest}
}

// Discover attestation bundles.
func (d *BundleDiscoveryMethod) Discover(ctx context.Context) ([]*workloadidentityv1.SigstoreVerificationPayload, error) {
	referrers, err := d.repo.Referrers(ctx, d.digest)
	if err != nil {
		return nil, trace.Wrap(err, "finding referrers")
	}

	payloads := make([]*workloadidentityv1.SigstoreVerificationPayload, 0, len(referrers.Manifests))
	for _, desc := range referrers.Manifests {
		if !strings.HasPrefix(desc.ArtifactType, mediaTypePrefixBundle) {
			continue
		}

		manifest, err := d.repo.Manifest(ctx, Digest(desc.Digest.String()))
		if err != nil {
			return nil, err
		}

		if l := len(manifest.Layers); l != 1 {
			return nil, trace.Errorf("expected manifest to have 1 layer, got: %d", l)
		}

		bundleJSON, err := d.repo.Layer(ctx, manifest.Layers[0].Digest)
		if err != nil {
			return nil, trace.Wrap(err, "pulling attestation layer")
		}

		var bundle bundlepb.Bundle
		if err := (protojson.UnmarshalOptions{}).Unmarshal(bundleJSON, &bundle); err != nil {
			return nil, trace.Wrap(err, "unmarshaling bundle JSON")
		}
		bundleProto, err := proto.Marshal(&bundle)
		if err != nil {
			return nil, trace.Wrap(err, "marshaling bundle proto")
		}

		payloads = append(payloads, &workloadidentityv1.SigstoreVerificationPayload{
			Bundle: bundleProto,
		})
		if len(payloads) == maxPayloads {
			break
		}
	}
	return payloads, nil
}
