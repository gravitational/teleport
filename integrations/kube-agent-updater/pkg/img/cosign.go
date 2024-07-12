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

package img

import (
	"context"
	"crypto"
	"encoding/hex"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/distribution/reference"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/gravitational/trace"
	"github.com/opencontainers/go-digest"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	ociremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
)

// hashAlgo is the Digest algorithm for OCI artfiacts:
// https://github.com/opencontainers/image-spec/blob/main/descriptor.md#digests
// SHA-256 is the de-facto standard, and we should be able to do everything with
// it: https://github.com/opencontainers/image-spec/blob/main/descriptor.md#sha-256
const hashAlgo = crypto.SHA256

type cosignKeyValidator struct {
	verifier        signature.Verifier
	skid            []byte
	name            string
	registryOptions []ociremote.Option
}

// Name returns the validator name, it is composed of a pretty name chosen at creation
// and its public SubjectKeyID hex-encoded.
func (v *cosignKeyValidator) Name() string {
	prettySKID := hex.EncodeToString(v.skid)
	return v.name + "-" + prettySKID
}

// TODO: cache this to protect against registry quotas
// The image validation is only invoked when we are in a maintenance window and
// the target version is different than our current version. In regular usage we
// are called only once per update. However, Kubernetes controllers failure mode
// is usually infinite retry loop. If something fails after the image validation,
// we might get called in a loop indefinitely. To mitigate the impact of such
// failure, ValidateAndResolveDigest should cache its result.

// ValidateAndResolveDigest resolves the image digest and validates it was
// signed with cosign using a trusted static key.
func (v *cosignKeyValidator) ValidateAndResolveDigest(ctx context.Context, image reference.NamedTagged) (NamedTaggedDigested, error) {
	checkOpts := &cosign.CheckOpts{
		RegistryClientOpts: v.registryOptions,
		Annotations:        nil,
		ClaimVerifier:      cosign.SimpleClaimVerifier,
		SigVerifier:        v.verifier,
		IgnoreTlog:         true, // TODO: should we keep this?
	}
	// Those are debug logs only
	log := ctrllog.FromContext(ctx).V(1)
	log.Info("Resolving digest", "image", image.String())

	ref, err := NamedTaggedToDigest(image, v.registryOptions...)
	if err != nil {
		return nil, trace.Wrap(err, "failed to resolve image digest")
	}
	log.Info("Resolved digest", "image", image.String(), "digest", ref.Digest, "reference", ref.String())

	verified, _, err := cosign.VerifyImageSignatures(ctx, ref, checkOpts)
	if err != nil {
		return nil, trace.Wrap(err, "failed to verify image signature")
	}
	if len(verified) == 0 {
		return nil, trace.Wrap(&trace.TrustError{Message: "cannot validate image: no valid signature found"})
	}
	log.Info("Signature validated", "image", image.String(), "digest", ref.Digest, "reference", ref.String())

	// There are legitimate use-cases where the signing reference is not the same
	// as the img we're pulling from: img promoted to an internal registry,
	// cache, registry migration, ...
	// Thus, we take only take the digest from the signature and use it with our base img.
	// This is what cosign.SimpleClaimVerifier is doing anyway.
	digestedImage := NewImageRef(ref.RegistryStr(), ref.RepositoryStr(), image.Tag(), digest.Digest(ref.DigestStr()))
	return digestedImage, nil
}

// NewCosignSingleKeyValidator takes a PEM-encoded public key and returns an
// img.Validator that checks the image was signed with cosign by the
// corresponding private key.
func NewCosignSingleKeyValidator(pem []byte, name string) (Validator, error) {
	pubKey, err := cryptoutils.UnmarshalPEMToPublicKey(pem)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	skid, err := cryptoutils.SKID(pubKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	verifier, err := signature.LoadVerifier(pubKey, hashAlgo)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &cosignKeyValidator{
		verifier: verifier,
		skid:     skid,
		name:     name,
	}, nil
}

// NamedTaggedToDigest resolves the digest of a reference.NamedTagged image and
// returns a name.Digest image corresponding to the resolved image.
func NamedTaggedToDigest(image reference.NamedTagged, opts ...ociremote.Option) (name.Digest, error) {
	ref, err := name.ParseReference(image.String())
	if err != nil {
		return name.Digest{}, trace.Wrap(err)
	}
	digested, err := ociremote.ResolveDigest(ref, opts...)
	return digested, trace.Wrap(err)
}
