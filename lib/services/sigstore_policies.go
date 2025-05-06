// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package services

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"regexp"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sigstore/sigstore-go/pkg/root"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
)

// SigstorePolicies is an interface over the SigstorePolicy service. This
// interface may also be implemented by a client to allow remote and local
// consumers to access the resource in a similar way.
type SigstorePolicies interface {
	// GetSigstorePolicy gets a SigstorePolicy by name.
	GetSigstorePolicy(
		ctx context.Context, name string,
	) (*workloadidentityv1pb.SigstorePolicy, error)

	// ListSigtorePolicies lists all SigstorePolicy resources using Google style
	// pagination.
	ListSigstorePolicies(
		ctx context.Context, pageSize int, lastToken string,
	) ([]*workloadidentityv1pb.SigstorePolicy, string, error)

	// CreateSigstorePolicy creates a new SigstorePolicy.
	CreateSigstorePolicy(
		ctx context.Context,
		sigstorePolicy *workloadidentityv1pb.SigstorePolicy,
	) (*workloadidentityv1pb.SigstorePolicy, error)

	// DeleteSigstorePolicy deletes a SigstorePolicy by name.
	DeleteSigstorePolicy(ctx context.Context, name string) error

	// UpdateSigstorePolicy updates a specific SigstorePolicy. The resource must
	// already exist, and, conditional update semantics are used - e.g the
	// submitted resource must have a revision matching the revision of the
	// resource in the backend.
	UpdateSigstorePolicy(
		ctx context.Context,
		sigstorePolicy *workloadidentityv1pb.SigstorePolicy,
	) (*workloadidentityv1pb.SigstorePolicy, error)

	// UpsertSigstorePolicy creates or updates a SigstorePolicy.
	UpsertSigstorePolicy(
		ctx context.Context,
		sigstorePolicy *workloadidentityv1pb.SigstorePolicy,
	) (*workloadidentityv1pb.SigstorePolicy, error)
}

// MarshalSigstorePolicy marshals the SigstorePolicy object into a JSON byte
// slice.
func MarshalSigstorePolicy(
	object *workloadidentityv1pb.SigstorePolicy, opts ...MarshalOption,
) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalSigstorePolicy unmarshals the SigstorePolicy object from a JSON byte
// slice.
func UnmarshalSigstorePolicy(
	data []byte, opts ...MarshalOption,
) (*workloadidentityv1pb.SigstorePolicy, error) {
	return UnmarshalProtoResource[*workloadidentityv1pb.SigstorePolicy](data, opts...)
}

// ValidateSigstorePolicy validates the SigstorePolicy object.
func ValidateSigstorePolicy(s *workloadidentityv1pb.SigstorePolicy) error {
	switch {
	case s.GetKind() != types.KindSigstorePolicy:
		return trace.BadParameter("kind: must be %q", types.KindSigstorePolicy)
	case s.GetSubKind() != "":
		return trace.BadParameter("sub_kind: must be empty")
	case s.GetVersion() != types.V1:
		return trace.BadParameter("version: only %q is supported", types.V1)
	case s.GetMetadata() == nil:
		return trace.BadParameter("metadata: is required")
	case s.GetMetadata().GetName() == "":
		return trace.BadParameter("metadata.name: is required")
	case s.GetSpec() == nil:
		return trace.BadParameter("spec: is required")
	case s.GetSpec().GetKey() == nil && s.GetSpec().GetKeyless() == nil:
		return trace.BadParameter("spec.authority: key or keyless authority is required")
	case s.GetSpec().GetRequirements() == nil:
		return trace.BadParameter("spec.requirements: is required")
	}

	switch v := s.GetSpec().GetAuthority().(type) {
	case *workloadidentityv1pb.SigstorePolicySpec_Key:
		public := v.Key.GetPublic()
		if public == "" {
			return trace.BadParameter("spec.key.public: is required")
		}

		block, rest := pem.Decode([]byte(public))
		if block == nil {
			return trace.BadParameter("spec.key.public: is not PEM encoded")
		}
		if !strings.Contains(block.Type, "PUBLIC KEY") {
			return trace.BadParameter("spec.key.public: must contain a public key, not: '%s'", block.Type)
		}
		if len(bytes.TrimSpace(rest)) != 0 {
			return trace.BadParameter("spec.key.public: must contain exactly one public key")
		}
		if _, err := x509.ParsePKIXPublicKey(block.Bytes); err != nil {
			return trace.BadParameter("spec.key.public: failed to parse public key: %v", err)
		}
	case *workloadidentityv1pb.SigstorePolicySpec_Keyless:
		if len(v.Keyless.GetIdentities()) == 0 {
			return trace.BadParameter("spec.keyless.identities: at least one trusted identity is required")
		}

		for idx, identity := range v.Keyless.GetIdentities() {
			switch {
			case identity.GetIssuer() != "":
			case identity.GetIssuerRegex() != "":
				if _, err := regexp.Compile(identity.GetIssuerRegex()); err != nil {
					return trace.BadParameter("spec.keyless.identities[%d].issuer_regex: failed to parse regex: %v", idx, err)
				}
			default:
				return trace.BadParameter("spec.keyless.identities[%d].issuer_matcher: issuer or issuer_regex is required", idx)
			}

			switch {
			case identity.GetSubject() != "":
			case identity.GetSubjectRegex() != "":
				if _, err := regexp.Compile(identity.GetSubjectRegex()); err != nil {
					return trace.BadParameter("spec.keyless.identities[%d].subject_regex: failed to parse regex: %v", idx, err)
				}
			default:
				return trace.BadParameter("spec.keyless.identities[%d].subject_matcher: subject or subject_regex is required", idx)
			}
		}

		roots := make(root.TrustedMaterialCollection, 0)
		for idx, trustedRoot := range v.Keyless.GetTrustedRoots() {
			root, err := root.NewTrustedRootFromJSON([]byte(trustedRoot))
			if err != nil {
				return trace.BadParameter("spec.keyless.trusted_roots[%d]: failed to parse trusted root: %v", idx, err)
			}
			roots = append(roots, root)
		}

		// If the user is overriding the default (Public Good Instance) trusted
		// roots with their own, they must specify at least one transparency log
		// or timestamp authority that can be used to verify keyless certificates.
		if len(roots) != 0 && len(roots.CTLogs()) == 0 && len(roots.TimestampingAuthorities()) == 0 {
			return trace.BadParameter("spec.keyless.trusted_roots: must configure at least one transparency log or timestamp authority")
		}
	}

	requirements := s.GetSpec().GetRequirements()

	if requirements.GetArtifactSignature() && len(requirements.GetAttestations()) != 0 {
		return trace.BadParameter("spec.requirements: artifact_signature and attestations are mutually exclusive")
	}

	if !requirements.GetArtifactSignature() && len(requirements.GetAttestations()) == 0 {
		return trace.BadParameter("spec.requirements: either artifact_signature or attestations is required")
	}

	for idx, attestation := range requirements.GetAttestations() {
		if attestation.GetPredicateType() == "" {
			return trace.BadParameter("spec.requirements.attestations[%d].predicate_type: is required", idx)
		}
	}

	return nil
}
