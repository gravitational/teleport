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
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/gravitational/trace"
	bundlepb "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	commonpb "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	rekorpb "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"google.golang.org/protobuf/proto"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

const (
	// MIME type for the simple signing envelope used by cosign when signing
	// OCI image digests.
	mediaTypeSimpleSigningEnvelope = "application/vnd.dev.cosign.simplesigning.v1+json"

	// annotations attached to the simple signing layer by Cosign which contain
	// the actual signature, certificate, chain, and Rekor metadata.
	annotationKeySignature   = "dev.cosignproject.cosign/signature"
	annotationKeyCertificate = "dev.sigstore.cosign/certificate"
	annotationKeyChain       = "dev.sigstore.cosign/chain"
	annotationKeyRekorBundle = "dev.sigstore.cosign/bundle"
)

// v0.1 of the bundle media type allows an InclusionPromise (SignedEntryTimestamp)
// instead of an InclusionProof, which we do not have when constructing a bundle
// from the annotations on a simple signing envelope layer.
var mediaTypeBundleV01 = func() string {
	t, err := bundle.MediaTypeString("0.1")
	if err != nil {
		panic(fmt.Sprintf("sigstore: failed to get bundle media type: %v", err))
	}
	return t
}()

// CosignSignatureDiscoveryMethod discovers image signatures created by cosign,
// represented in the old (pre-bundle) format based on the single signing
// envelope format.
//
// https://github.com/sigstore/cosign/blob/37bae90768f66c930b5630d0f570778141878737/specs/SIGNATURE_SPEC.md
type CosignSignatureDiscoveryMethod struct {
	repo   *Repository
	digest v1.Hash
}

// NewCosignSignatureDiscoveryMethod creates a new CosignSignatureDiscoveryMethod
// with the given repo and digest.
func NewCosignSignatureDiscoveryMethod(repo *Repository, digest v1.Hash) *CosignSignatureDiscoveryMethod {
	return &CosignSignatureDiscoveryMethod{repo, digest}
}

// Discover cosign image signatures.
func (d *CosignSignatureDiscoveryMethod) Discover(ctx context.Context) ([]*workloadidentityv1.SigstoreVerificationPayload, error) {
	tag := fmt.Sprintf("%s-%s.sig", d.digest.Algorithm, d.digest.Hex)

	mf, err := d.repo.Manifest(ctx, Tag(tag))
	var te *transport.Error
	switch {
	case errors.As(err, &te) && te.StatusCode == http.StatusNotFound:
		return nil, nil
	case err != nil:
		return nil, trace.Wrap(err, "pulling manifest")
	}

	payloads := make([]*workloadidentityv1.SigstoreVerificationPayload, 0, len(mf.Layers))
	for _, desc := range mf.Layers {
		if desc.MediaType != mediaTypeSimpleSigningEnvelope {
			continue
		}

		// Construct a bundle from the layer annotations.
		bundle, err := d.constructBundle(ctx, desc)
		if err != nil {
			return nil, trace.Wrap(err, "building simple signing bundle")
		}

		bundleBytes, err := proto.Marshal(bundle)
		if err != nil {
			return nil, trace.Wrap(err, "marshaling bundle to protobuf")
		}

		// Pull the simple signing envelope so we can send it to the server.
		envelope, err := d.repo.Layer(ctx, desc.Digest)
		if err != nil {
			return nil, trace.Wrap(err, "pulling simple signing layer")
		}

		payloads = append(payloads, &workloadidentityv1.SigstoreVerificationPayload{
			Bundle:                bundleBytes,
			SimpleSigningEnvelope: envelope,
		})
		if len(payloads) == maxPayloads {
			break
		}
	}
	return payloads, nil
}

func (d *CosignSignatureDiscoveryMethod) constructBundle(ctx context.Context, desc v1.Descriptor) (*bundlepb.Bundle, error) {
	vm, err := d.gatherVerificationMaterial(ctx, desc)
	if err != nil {
		return nil, trace.Wrap(err, "gathering verification material")
	}
	sig, err := d.parseSignature(ctx, desc)
	if err != nil {
		return nil, trace.Wrap(err, "parsing signature")
	}

	return &bundlepb.Bundle{
		MediaType:            mediaTypeBundleV01,
		VerificationMaterial: vm,
		Content:              sig,
	}, nil
}

func (d *CosignSignatureDiscoveryMethod) gatherVerificationMaterial(ctx context.Context, desc v1.Descriptor) (*bundlepb.VerificationMaterial, error) {
	vm := &bundlepb.VerificationMaterial{}
	certs := make([]*commonpb.X509Certificate, 0)

	if a := desc.Annotations[annotationKeyCertificate]; a != "" {
		block, _ := pem.Decode([]byte(a))
		if block == nil {
			return nil, trace.Errorf("'%s' annotation contains malformed certificate", annotationKeyCertificate)
		}
		certs = append(certs, &commonpb.X509Certificate{RawBytes: block.Bytes})
	}

	if a := desc.Annotations[annotationKeyChain]; a != "" {
		if len(certs) == 0 {
			return nil, trace.Errorf("'%s' annotation present without '%s' annotation", annotationKeyChain, annotationKeyCertificate)
		}
		for block, rest := pem.Decode([]byte(a)); block != nil; block, rest = pem.Decode(rest) {
			certs = append(certs, &commonpb.X509Certificate{RawBytes: block.Bytes})
		}
	}

	switch len(certs) {
	case 0:
		vm.Content = &bundlepb.VerificationMaterial_PublicKey{
			PublicKey: &commonpb.PublicKeyIdentifier{},
		}
	case 1:
		vm.Content = &bundlepb.VerificationMaterial_Certificate{
			Certificate: certs[0],
		}
	default:
		vm.Content = &bundlepb.VerificationMaterial_X509CertificateChain{
			X509CertificateChain: &commonpb.X509CertificateChain{
				Certificates: certs,
			},
		}
	}

	if a := desc.Annotations[annotationKeyRekorBundle]; a != "" {
		logEntry, err := d.parseRekorBundle(ctx, []byte(a))
		if err != nil {
			return nil, trace.Wrap(err, "parsing Rekor bundle")
		}
		vm.TlogEntries = []*rekorpb.TransparencyLogEntry{logEntry}
	}

	return vm, nil
}

func (d *CosignSignatureDiscoveryMethod) parseRekorBundle(ctx context.Context, annotation []byte) (*rekorpb.TransparencyLogEntry, error) {
	// Note: bundle here refers to the old Rekor bundle format, *not* the new
	// universal Sigstore bundle format.
	var bundle struct {
		SignedEntryTimestamp []byte `json:"SignedEntryTimestamp"`

		Payload struct {
			Body           []byte `json:"body"`
			IntegratedTime int64  `json:"integratedTime"`
			LogIndex       int64  `json:"logIndex"`
			LogID          string `json:"logID"`
		} `json:"Payload"`
	}
	if err := json.Unmarshal(annotation, &bundle); err != nil {
		return nil, trace.Wrap(err, "unmarshaling annotation")
	}

	logID, err := hex.DecodeString(bundle.Payload.LogID)
	if err != nil {
		return nil, trace.Wrap(err, "decoding log id")
	}

	var kindVersion struct {
		Kind       string `json:"kind"`
		APIVersion string `json:"apiVersion"`
	}
	if err := json.Unmarshal(bundle.Payload.Body, &kindVersion); err != nil {
		return nil, trace.Wrap(err, "unmarshaling kind/version")
	}

	return &rekorpb.TransparencyLogEntry{
		LogIndex: bundle.Payload.LogIndex,
		LogId: &commonpb.LogId{
			KeyId: logID,
		},
		KindVersion: &rekorpb.KindVersion{
			Kind:    kindVersion.Kind,
			Version: kindVersion.APIVersion,
		},
		IntegratedTime: bundle.Payload.IntegratedTime,
		InclusionPromise: &rekorpb.InclusionPromise{
			SignedEntryTimestamp: bundle.SignedEntryTimestamp,
		},
		CanonicalizedBody: bundle.Payload.Body,
	}, nil
}

func (d *CosignSignatureDiscoveryMethod) parseSignature(ctx context.Context, desc v1.Descriptor) (*bundlepb.Bundle_MessageSignature, error) {
	if alg := desc.Digest.Algorithm; alg != "sha256" {
		return nil, trace.Errorf("unsupported digest algorithm: %s", alg)
	}

	annotation := desc.Annotations[annotationKeySignature]
	if annotation == "" {
		return nil, trace.BadParameter("annotation '%s' is missing", annotationKeySignature)
	}

	sig, err := base64.StdEncoding.DecodeString(annotation)
	if err != nil {
		return nil, trace.Wrap(err, "base64-decoding signature annotation")
	}

	digest, err := hex.DecodeString(desc.Digest.Hex)
	if err != nil {
		return nil, trace.Wrap(err, "hex-decoding layer digest")
	}

	return &bundlepb.Bundle_MessageSignature{
		MessageSignature: &commonpb.MessageSignature{
			Signature: sig,
			MessageDigest: &commonpb.HashOutput{
				Algorithm: commonpb.HashAlgorithm_SHA2_256,
				Digest:    digest,
			},
		},
	}, nil
}
