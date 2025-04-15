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

package services_test

import (
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestValidateSigstorePolicy(t *testing.T) {
	require.NoError(t, services.ValidateSigstorePolicy(validSigstorePolicy()))

	testCases := map[string]struct {
		mod func(*workloadidentityv1.SigstorePolicy)
		err string
	}{
		"no metadata": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Metadata = nil
			},
			err: "metadata: is required",
		},
		"no name": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Metadata.Name = ""
			},
			err: "metadata.name: is required",
		},
		"invalid version": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Version = "1000000"
			},
			err: `version: only "v1" is supported`,
		},
		"invalid kind": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Kind = types.KindWorkloadIdentity
			},
			err: `kind: must be "sigstore_policy`,
		},
		"invalid sub_kind": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.SubKind = "hello world"
			},
			err: `sub_kind: must be empty`,
		},
		"no spec": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Spec = nil
			},
			err: "spec: is required",
		},
		"no authority": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Spec.Authority = nil
			},
			err: "spec.authority: key or keyless authority is required",
		},
		"empty key authority": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Spec.Authority = &workloadidentityv1.SigstorePolicySpec_Key{}
			},
			err: "spec.authority: key or keyless authority is required",
		},
		"empty keyless authority": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Spec.Authority = &workloadidentityv1.SigstorePolicySpec_Keyless{}
			},
			err: "spec.authority: key or keyless authority is required",
		},
		"no public key": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Spec.Authority = &workloadidentityv1.SigstorePolicySpec_Key{
					Key: &workloadidentityv1.SigstoreKeyAuthority{
						Public: "",
					},
				}
			},
			err: "spec.key.public: is required",
		},
		"public key is not PEM": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Spec.Authority = &workloadidentityv1.SigstorePolicySpec_Key{
					Key: &workloadidentityv1.SigstoreKeyAuthority{
						Public: "NOT PEM",
					},
				}
			},
			err: "spec.key.public: is not PEM encoded",
		},
		"public key is not a public key": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Spec.Authority = &workloadidentityv1.SigstorePolicySpec_Key{
					Key: &workloadidentityv1.SigstoreKeyAuthority{
						Public: "\n-----BEGIN RSA PRIVATE KEY-----\n-----END RSA PRIVATE KEY-----\n",
					},
				}
			},
			err: "spec.key.public: must contain a public key, not: 'RSA PRIVATE KEY'",
		},
		"public key contains more than one key": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Spec.Authority = &workloadidentityv1.SigstorePolicySpec_Key{
					Key: &workloadidentityv1.SigstoreKeyAuthority{
						Public: strings.Repeat("\n-----BEGIN PUBLIC KEY-----\n-----END PUBLIC KEY-----\n", 2),
					},
				}
			},
			err: "spec.key.public: must contain exactly one public key",
		},
		"public key is malformed": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Spec.Authority = &workloadidentityv1.SigstorePolicySpec_Key{
					Key: &workloadidentityv1.SigstoreKeyAuthority{
						Public: "\n-----BEGIN PUBLIC KEY-----\nYm9ndXMK\n-----END PUBLIC KEY-----\n",
					},
				}
			},
			err: "spec.key.public: failed to parse public key: asn1: structure error",
		},
		"no keyless authorities": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Spec.GetKeyless().Identities = nil
			},
			err: "spec.keyless.identities: at least one trusted identity is required",
		},
		"no keyless identity subject": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Spec.GetKeyless().Identities = []*workloadidentityv1.SigstoreKeylessSigningIdentity{
					{
						SubjectMatcher: nil,
						IssuerMatcher: &workloadidentityv1.SigstoreKeylessSigningIdentity_Issuer{
							Issuer: "some issuer",
						},
					},
				}
			},
			err: "spec.keyless.identities[0].subject_matcher: subject or subject_regex is required",
		},
		"keyless identity subject_regex invalid": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Spec.GetKeyless().Identities = []*workloadidentityv1.SigstoreKeylessSigningIdentity{
					{
						SubjectMatcher: &workloadidentityv1.SigstoreKeylessSigningIdentity_SubjectRegex{
							SubjectRegex: `(abc[def`,
						},
						IssuerMatcher: &workloadidentityv1.SigstoreKeylessSigningIdentity_Issuer{
							Issuer: "some issuer",
						},
					},
				}
			},
			err: "spec.keyless.identities[0].subject_regex: failed to parse regex",
		},
		"no keyless identity issuer": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Spec.GetKeyless().Identities = []*workloadidentityv1.SigstoreKeylessSigningIdentity{
					{
						IssuerMatcher: nil,
						SubjectMatcher: &workloadidentityv1.SigstoreKeylessSigningIdentity_Subject{
							Subject: "some subject",
						},
					},
				}
			},
			err: "spec.keyless.identities[0].issuer_matcher: issuer or issuer_regex is required",
		},
		"keyless identity issuer_regex invalid": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Spec.GetKeyless().Identities = []*workloadidentityv1.SigstoreKeylessSigningIdentity{
					{
						IssuerMatcher: &workloadidentityv1.SigstoreKeylessSigningIdentity_IssuerRegex{
							IssuerRegex: `(abc[def`,
						},
						SubjectMatcher: &workloadidentityv1.SigstoreKeylessSigningIdentity_Subject{
							Subject: "some subject",
						},
					},
				}
			},
			err: "spec.keyless.identities[0].issuer_regex: failed to parse regex",
		},
		"keyless invalid trusted_roots": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Spec.GetKeyless().TrustedRoots = []string{`{}`}
			},
			err: "spec.keyless.trusted_roots[0]: failed to parse trusted root",
		},
		"no requirements": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Spec.Requirements = nil
			},
			err: "spec.requirements: is required",
		},
		"required attestation empty predicate": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.Spec.Requirements.Attestations = []*workloadidentityv1.InTotoAttestationMatcher{
					{PredicateType: ""},
				}
			},
			err: "spec.requirements.attestations[0].predicate_type: is required",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			policy := validSigstorePolicy()
			tc.mod(policy)

			err := services.ValidateSigstorePolicy(policy)
			require.ErrorContains(t, err, tc.err)
			require.True(t, trace.IsBadParameter(err))
		})
	}
}

func validSigstorePolicy() *workloadidentityv1.SigstorePolicy {
	return &workloadidentityv1.SigstorePolicy{
		Kind:    types.KindSigstorePolicy,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "github-provenance",
		},
		Spec: &workloadidentityv1.SigstorePolicySpec{
			Authority: &workloadidentityv1.SigstorePolicySpec_Keyless{
				Keyless: &workloadidentityv1.SigstoreKeylessAuthority{
					Identities: []*workloadidentityv1.SigstoreKeylessSigningIdentity{
						{
							IssuerMatcher: &workloadidentityv1.SigstoreKeylessSigningIdentity_Issuer{
								Issuer: "https://token.actions.githubusercontent.com",
							},
							SubjectMatcher: &workloadidentityv1.SigstoreKeylessSigningIdentity_SubjectRegex{
								SubjectRegex: `https://github.com/mycompany/.*/\.github/workflows/.*@.*`,
							},
						},
					},
				},
			},
			Requirements: &workloadidentityv1.SigstorePolicyRequirements{
				Attestations: []*workloadidentityv1.InTotoAttestationMatcher{
					{PredicateType: "https://slsa.dev/provenance/v1"},
				},
			},
		},
	}
}
