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
	"google.golang.org/protobuf/proto"
)

func TestValidateSigstorePolicy(t *testing.T) {
	require.NoError(t, services.ValidateSigstorePolicy(validSigstorePolicy()))

	testCases := map[string]struct {
		mod func(*workloadidentityv1.SigstorePolicy)
		err string
	}{
		"no metadata": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.ClearMetadata()
			},
			err: "metadata: is required",
		},
		"no name": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.GetMetadata().Name = ""
			},
			err: "metadata.name: is required",
		},
		"invalid version": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.SetVersion("1000000")
			},
			err: `version: only "v1" is supported`,
		},
		"invalid kind": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.SetKind(types.KindWorkloadIdentity)
			},
			err: `kind: must be "sigstore_policy`,
		},
		"invalid sub_kind": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.SetSubKind("hello world")
			},
			err: `sub_kind: must be empty`,
		},
		"no spec": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.ClearSpec()
			},
			err: "spec: is required",
		},
		"no authority": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.GetSpec().ClearAuthority()
			},
			err: "spec.authority: key or keyless authority is required",
		},
		"no public key": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.GetSpec().SetKey(workloadidentityv1.SigstoreKeyAuthority_builder{
					Public: "",
				}.Build())
			},
			err: "spec.key.public: is required",
		},
		"public key is not PEM": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.GetSpec().SetKey(workloadidentityv1.SigstoreKeyAuthority_builder{
					Public: "NOT PEM",
				}.Build())
			},
			err: "spec.key.public: is not PEM encoded",
		},
		"public key is not a public key": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.GetSpec().SetKey(workloadidentityv1.SigstoreKeyAuthority_builder{
					Public: "\n-----BEGIN RSA PRIVATE KEY-----\n-----END RSA PRIVATE KEY-----\n",
				}.Build())
			},
			err: "spec.key.public: must contain a public key, not: 'RSA PRIVATE KEY'",
		},
		"public key contains more than one key": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.GetSpec().SetKey(workloadidentityv1.SigstoreKeyAuthority_builder{
					Public: strings.Repeat("\n-----BEGIN PUBLIC KEY-----\n-----END PUBLIC KEY-----\n", 2),
				}.Build())
			},
			err: "spec.key.public: must contain exactly one public key",
		},
		"public key is malformed": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.GetSpec().SetKey(workloadidentityv1.SigstoreKeyAuthority_builder{
					Public: "\n-----BEGIN PUBLIC KEY-----\nYm9ndXMK\n-----END PUBLIC KEY-----\n",
				}.Build())
			},
			err: "spec.key.public: failed to parse public key: asn1: structure error",
		},
		"no keyless authorities": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.GetSpec().GetKeyless().SetIdentities(nil)
			},
			err: "spec.keyless.identities: at least one trusted identity is required",
		},
		"no keyless identity subject": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.GetSpec().GetKeyless().SetIdentities([]*workloadidentityv1.SigstoreKeylessSigningIdentity{
					workloadidentityv1.SigstoreKeylessSigningIdentity_builder{
						Issuer: proto.String("some issuer"),
					}.Build(),
				})
			},
			err: "spec.keyless.identities[0].subject_matcher: subject or subject_regex is required",
		},
		"keyless identity subject_regex invalid": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.GetSpec().GetKeyless().SetIdentities([]*workloadidentityv1.SigstoreKeylessSigningIdentity{
					workloadidentityv1.SigstoreKeylessSigningIdentity_builder{
						SubjectRegex: proto.String(`(abc[def`),
						Issuer:       proto.String("some issuer"),
					}.Build(),
				})
			},
			err: "spec.keyless.identities[0].subject_regex: failed to parse regex",
		},
		"no keyless identity issuer": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.GetSpec().GetKeyless().SetIdentities([]*workloadidentityv1.SigstoreKeylessSigningIdentity{
					workloadidentityv1.SigstoreKeylessSigningIdentity_builder{
						Subject: proto.String("some subject"),
					}.Build(),
				})
			},
			err: "spec.keyless.identities[0].issuer_matcher: issuer or issuer_regex is required",
		},
		"keyless identity issuer_regex invalid": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.GetSpec().GetKeyless().SetIdentities([]*workloadidentityv1.SigstoreKeylessSigningIdentity{
					workloadidentityv1.SigstoreKeylessSigningIdentity_builder{
						IssuerRegex: proto.String(`(abc[def`),
						Subject:     proto.String("some subject"),
					}.Build(),
				})
			},
			err: "spec.keyless.identities[0].issuer_regex: failed to parse regex",
		},
		"keyless invalid trusted_roots": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.GetSpec().GetKeyless().SetTrustedRoots([]string{`{}`})
			},
			err: "spec.keyless.trusted_roots[0]: failed to parse trusted root",
		},
		"keyless trusted_roots contains no tlogs or timestampAuthorities": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				// This is GitHub's trusted roots with the `tlogs` and timestampAuthorities`
				// lists blanked out.
				const root = `
				{
				  "mediaType": "application/vnd.dev.sigstore.trustedroot+json;version=0.1",
				  "certificateAuthorities": [
				    {
				      "subject": {
				        "organization": "GitHub, Inc.",
				        "commonName": "Internal Services Root"
				      },
				      "uri": "fulcio.githubapp.com",
				      "certChain": {
				        "certificates": [
				          {
				            "rawBytes": "MIICKzCCAbCgAwIBAgIUQeyd9UH06yZ63pDuqjgUZ58CnpMwCgYIKoZIzj0EAwMwODEVMBMGA1UEChMMR2l0SHViLCBJbmMuMR8wHQYDVQQDExZGdWxjaW8gSW50ZXJtZWRpYXRlIGwxMB4XDTI0MTAwMzEyMDAwMFoXDTI1MTAwMzEyMDAwMFowODEVMBMGA1UEChMMR2l0SHViLCBJbmMuMR8wHQYDVQQDExZGdWxjaW8gSW50ZXJtZWRpYXRlIGwyMHYwEAYHKoZIzj0CAQYFK4EEACIDYgAEwvbET2w+j9j9j50iTInH1gb9GSXkpsCvWz5orX1zgme+/Qh/5gMkpfmgfOSLV2ZRgT1hzujYmnKQvP2mCxYnbwQELAkAf+VhEY/7Uw3zZvguGQSdF1cxzRHiMTOha5eFo3sweTAOBgNVHQ8BAf8EBAMCAQYwEwYDVR0lBAwwCgYIKwYBBQUHAwMwEgYDVR0TAQH/BAgwBgEB/wIBADAdBgNVHQ4EFgQUMib9z4ZYBcQANTVvVCa3KoTGbBUwHwYDVR0jBBgwFoAUwOG4UqRLTz7eejgRBs9JjqFFmzMwCgYIKoZIzj0EAwMDaQAwZgIxAPIU/zlJiJrxn6oTWNdEAD/YBSnhyxcvpq1D2DzFy8E8hbkEfMZPErYL7HyoL/BkdwIxAN9KDEKyktEUBrfHehfcLAzI2kERJx+8DSslXswOIbLaeqYfWsmrQAt5C0X/nOWxXA=="
				          },
				          {
				            "rawBytes": "MIICFTCCAZugAwIBAgIUD3Jlqt4qhrcZI4UnGfPGrEq/pjQwCgYIKoZIzj0EAwMwODEVMBMGA1UEChMMR2l0SHViLCBJbmMuMR8wHQYDVQQDExZJbnRlcm5hbCBTZXJ2aWNlcyBSb290MB4XDTIzMDkxMTEyMDAwMFoXDTI4MDkwOTEyMDAwMFowODEVMBMGA1UEChMMR2l0SHViLCBJbmMuMR8wHQYDVQQDExZGdWxjaW8gSW50ZXJtZWRpYXRlIGwxMHYwEAYHKoZIzj0CAQYFK4EEACIDYgAE7X7nK0wC7uEmDjW+on0sXIX3FacL3hhcrhneA+M/kl1OtvQiPmFrH9lbUQqOj/AfspJ8uGY3jaq8WuSg6ghatzYfuuzLAJIK4nGpCBafncF8EynOssPq64/Dz+JUWXqlo2YwZDAOBgNVHQ8BAf8EBAMCAQYwEgYDVR0TAQH/BAgwBgEB/wIBATAdBgNVHQ4EFgQUwOG4UqRLTz7eejgRBs9JjqFFmzMwHwYDVR0jBBgwFoAUfFJ5/6rhfHEZPnXAhrQLhGkJJMwwCgYIKoZIzj0EAwMDaAAwZQIxAI8HWLrke7uzhOpwlD1cNixPmoX9XFKe7bEPozo0D+vKi0Gt6VlC7xPedFIw4/AypAIwQP+FGRWvfx0IAH5/n0aRiN7/LVpyFA5RkJASZOVOib2Y8pNuhXa9V3ZbWO6v6kW/"
				          },
				          {
				            "rawBytes": "MIIB9TCCAXqgAwIBAgIUNFryA06EHDIcd5EIbe8swbl9OY4wCgYIKoZIzj0EAwMwODEVMBMGA1UEChMMR2l0SHViLCBJbmMuMR8wHQYDVQQDExZJbnRlcm5hbCBTZXJ2aWNlcyBSb290MB4XDTIzMDgwNzEyMDAwMFoXDTMzMDgwNDEyMDAwMFowODEVMBMGA1UEChMMR2l0SHViLCBJbmMuMR8wHQYDVQQDExZJbnRlcm5hbCBTZXJ2aWNlcyBSb290MHYwEAYHKoZIzj0CAQYFK4EEACIDYgAEXYaXx4H0oNuVP/2cfydA3oaafvvkkkgb5hbL8/j/BO25S7uTmDOCA5e4QLLWCKFuc+xp2j14tCH4WmHzMUDvf2tXtInVliY5wZgQMM9L6klo/IwA9x4omdcjnT+kKJAjo0UwQzAOBgNVHQ8BAf8EBAMCAQYwEgYDVR0TAQH/BAgwBgEB/wIBAjAdBgNVHQ4EFgQUfFJ5/6rhfHEZPnXAhrQLhGkJJMwwCgYIKoZIzj0EAwMDaQAwZgIxAPzXsV+eokrqOHSQZH/XhhHE1slOscKy3DQpYpYJ1AWmJ2lJu/XOmubBX5s7apllUwIxALw2Ts8CDACiK42UymC8fk6sbNfoXUAWqdyKTVt2Lst+wNdkRniGvx7jT65BKTkcsQ=="
				          }
				        ]
				      },
				      "validFor": {
				        "start": "2024-10-07T00:00:00Z"
				      }
				    }
				  ],
				  "timestampAuthorities": [],
				  "tlogs": []
				}
				`
				p.GetSpec().GetKeyless().SetTrustedRoots([]string{root})
			},
			err: "spec.keyless.trusted_roots: must configure at least one transparency log or timestamp authority",
		},
		"no requirements": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.GetSpec().ClearRequirements()
			},
			err: "spec.requirements: is required",
		},
		"empty requirements": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.GetSpec().SetRequirements(&workloadidentityv1.SigstorePolicyRequirements{})
			},
			err: "spec.requirements: either artifact_signature or attestations is required",
		},
		"required attestation empty predicate": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.GetSpec().GetRequirements().SetAttestations([]*workloadidentityv1.InTotoAttestationMatcher{
					workloadidentityv1.InTotoAttestationMatcher_builder{PredicateType: ""}.Build(),
				})
			},
			err: "spec.requirements.attestations[0].predicate_type: is required",
		},
		"attestations and artifact signature": {
			mod: func(p *workloadidentityv1.SigstorePolicy) {
				p.GetSpec().SetRequirements(workloadidentityv1.SigstorePolicyRequirements_builder{
					ArtifactSignature: true,
					Attestations: []*workloadidentityv1.InTotoAttestationMatcher{
						workloadidentityv1.InTotoAttestationMatcher_builder{PredicateType: "https://slsa.dev/provenance/v1"}.Build(),
					},
				}.Build())
			},
			err: "spec.requirements: artifact_signature and attestations are mutually exclusive",
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
	return workloadidentityv1.SigstorePolicy_builder{
		Kind:    types.KindSigstorePolicy,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "github-provenance",
		},
		Spec: workloadidentityv1.SigstorePolicySpec_builder{
			Keyless: workloadidentityv1.SigstoreKeylessAuthority_builder{
				Identities: []*workloadidentityv1.SigstoreKeylessSigningIdentity{
					workloadidentityv1.SigstoreKeylessSigningIdentity_builder{
						Issuer:       proto.String("https://token.actions.githubusercontent.com"),
						SubjectRegex: proto.String(`https://github.com/mycompany/.*/\.github/workflows/.*@.*`),
					}.Build(),
				},
			}.Build(),
			Requirements: workloadidentityv1.SigstorePolicyRequirements_builder{
				Attestations: []*workloadidentityv1.InTotoAttestationMatcher{
					workloadidentityv1.InTotoAttestationMatcher_builder{PredicateType: "https://slsa.dev/provenance/v1"}.Build(),
				},
			}.Build(),
		}.Build(),
	}.Build()
}
