package sigstore_test

import (
	"bytes"
	"context"
	_ "embed"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/e/lib/sigstore"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

var (
	//go:embed testdata/ecdsa.pub
	ecdsaPublicKey string

	//go:embed testdata/rsa.pub
	rsaPublicKey string

	//go:embed testdata/ed25519.pub
	ed25519PublicKey string

	//go:embed testdata/ecdsa_simple_signing_payload.json
	ecdsaSimpleSigningPayload []byte

	//go:embed testdata/rsa_simple_signing_payload.json
	rsaSimpleSigningPayload []byte

	//go:embed testdata/ed25519_simple_signing_payload.json
	ed25519SimpleSigningPayload []byte

	//go:embed testdata/keyless_simple_signing_payload.json
	keylessSimpleSigningPayload []byte

	//go:embed testdata/github_trusted_root.json
	githubTrustedRootJSON string

	//go:embed testdata/github_provenance_payload.json
	githubProvenancePayload []byte
)

func TestPolicyEvaluator(t *testing.T) {
	testCases := map[string]struct {
		policy *workloadidentityv1.SigstorePolicySpec
		attrs  *workloadidentityv1.WorkloadAttrs
		error  string
	}{
		"Success - ECDSA Keypair - Simple Signing Envelope": {
			policy: &workloadidentityv1.SigstorePolicySpec{
				Authority: &workloadidentityv1.SigstorePolicySpec_Key{
					Key: &workloadidentityv1.SigstoreKeyAuthority{
						Public: ecdsaPublicKey,
					},
				},
				Requirements: &workloadidentityv1.SigstorePolicyRequirements{
					ArtifactSignature: true,
				},
			},
			attrs: &workloadidentityv1.WorkloadAttrs{
				Docker: &workloadidentityv1.WorkloadAttrsDocker{
					Container: &workloadidentityv1.WorkloadAttrsDockerContainer{
						ImageDigest: "sha256:6f427c9b3350e16ce42c24548347e9078b3562a529df48318a9cc82eafba842c",
					},
				},
				Sigstore: &workloadidentityv1.WorkloadAttrsSigstore{
					Payloads: mustDecodePayloads(t, ecdsaSimpleSigningPayload),
				},
			},
		},
		"Success - RSA Keypair - Simple Signing Envelope": {
			policy: &workloadidentityv1.SigstorePolicySpec{
				Authority: &workloadidentityv1.SigstorePolicySpec_Key{
					Key: &workloadidentityv1.SigstoreKeyAuthority{
						Public: rsaPublicKey,
					},
				},
				Requirements: &workloadidentityv1.SigstorePolicyRequirements{
					ArtifactSignature: true,
				},
			},
			attrs: &workloadidentityv1.WorkloadAttrs{
				Docker: &workloadidentityv1.WorkloadAttrsDocker{
					Container: &workloadidentityv1.WorkloadAttrsDockerContainer{
						ImageDigest: "sha256:6f427c9b3350e16ce42c24548347e9078b3562a529df48318a9cc82eafba842c",
					},
				},
				Sigstore: &workloadidentityv1.WorkloadAttrsSigstore{
					Payloads: mustDecodePayloads(t, rsaSimpleSigningPayload),
				},
			},
		},
		"Success - ED25519 Keypair - Simple Signing Envelope": {
			policy: &workloadidentityv1.SigstorePolicySpec{
				Authority: &workloadidentityv1.SigstorePolicySpec_Key{
					Key: &workloadidentityv1.SigstoreKeyAuthority{
						Public: ed25519PublicKey,
					},
				},
				Requirements: &workloadidentityv1.SigstorePolicyRequirements{
					ArtifactSignature: true,
				},
			},
			attrs: &workloadidentityv1.WorkloadAttrs{
				Docker: &workloadidentityv1.WorkloadAttrsDocker{
					Container: &workloadidentityv1.WorkloadAttrsDockerContainer{
						ImageDigest: "sha256:6f427c9b3350e16ce42c24548347e9078b3562a529df48318a9cc82eafba842c",
					},
				},
				Sigstore: &workloadidentityv1.WorkloadAttrsSigstore{
					Payloads: mustDecodePayloads(t, ed25519SimpleSigningPayload),
				},
			},
		},
		"Failure - Key Mismatch": {
			policy: &workloadidentityv1.SigstorePolicySpec{
				Authority: &workloadidentityv1.SigstorePolicySpec_Key{
					Key: &workloadidentityv1.SigstoreKeyAuthority{
						Public: ecdsaPublicKey,
					},
				},
				Requirements: &workloadidentityv1.SigstorePolicyRequirements{
					ArtifactSignature: true,
				},
			},
			attrs: &workloadidentityv1.WorkloadAttrs{
				Docker: &workloadidentityv1.WorkloadAttrsDocker{
					Container: &workloadidentityv1.WorkloadAttrsDockerContainer{
						ImageDigest: "sha256:6f427c9b3350e16ce42c24548347e9078b3562a529df48318a9cc82eafba842c",
					},
				},
				Sigstore: &workloadidentityv1.WorkloadAttrsSigstore{
					Payloads: mustDecodePayloads(t, rsaSimpleSigningPayload), // <- Signed by a different key
				},
			},
			error: "Policy not satisfied by the given bundles",
		},
		"Success - Keyless Public Good - Simple Signing Envelope": {
			policy: &workloadidentityv1.SigstorePolicySpec{
				Authority: &workloadidentityv1.SigstorePolicySpec_Keyless{
					Keyless: &workloadidentityv1.SigstoreKeylessAuthority{
						Identities: []*workloadidentityv1.SigstoreKeylessSigningIdentity{
							{
								IssuerMatcher: &workloadidentityv1.SigstoreKeylessSigningIdentity_Issuer{
									Issuer: "https://accounts.google.com",
								},
								SubjectMatcher: &workloadidentityv1.SigstoreKeylessSigningIdentity_Subject{
									Subject: "daniel.upton@goteleport.com",
								},
							},
						},
					},
				},
				Requirements: &workloadidentityv1.SigstorePolicyRequirements{
					ArtifactSignature: true,
				},
			},
			attrs: &workloadidentityv1.WorkloadAttrs{
				Docker: &workloadidentityv1.WorkloadAttrsDocker{
					Container: &workloadidentityv1.WorkloadAttrsDockerContainer{
						ImageDigest: "sha256:6f427c9b3350e16ce42c24548347e9078b3562a529df48318a9cc82eafba842c",
					},
				},
				Sigstore: &workloadidentityv1.WorkloadAttrsSigstore{
					Payloads: mustDecodePayloads(t, keylessSimpleSigningPayload),
				},
			},
		},
		"Success - Keyless Custom Trusted Root - Attestations": {
			policy: &workloadidentityv1.SigstorePolicySpec{
				Authority: &workloadidentityv1.SigstorePolicySpec_Keyless{
					Keyless: &workloadidentityv1.SigstoreKeylessAuthority{
						Identities: []*workloadidentityv1.SigstoreKeylessSigningIdentity{
							{
								IssuerMatcher: &workloadidentityv1.SigstoreKeylessSigningIdentity_Issuer{
									Issuer: "https://token.actions.githubusercontent.com/teleport",
								},
								SubjectMatcher: &workloadidentityv1.SigstoreKeylessSigningIdentity_SubjectRegex{
									SubjectRegex: `(.*)/.github/workflows/build.yaml@refs/heads/main`,
								},
							},
						},
						TrustedRoots: []string{githubTrustedRootJSON},
					},
				},
				Requirements: &workloadidentityv1.SigstorePolicyRequirements{
					Attestations: []*workloadidentityv1.InTotoAttestationMatcher{
						{PredicateType: "https://slsa.dev/provenance/v1"},
					},
				},
			},
			attrs: &workloadidentityv1.WorkloadAttrs{
				Docker: &workloadidentityv1.WorkloadAttrsDocker{
					Container: &workloadidentityv1.WorkloadAttrsDockerContainer{
						ImageDigest: "sha256:c95f6bcc67ae1d482733526520d8bff0e35f4e258a6d0d615c0ed28dd86263e2",
					},
				},
				Sigstore: &workloadidentityv1.WorkloadAttrsSigstore{
					Payloads: mustDecodePayloads(t, githubProvenancePayload),
				},
			},
		},
		"Failure - Missing Attestation": {
			policy: &workloadidentityv1.SigstorePolicySpec{
				Authority: &workloadidentityv1.SigstorePolicySpec_Keyless{
					Keyless: &workloadidentityv1.SigstoreKeylessAuthority{
						Identities: []*workloadidentityv1.SigstoreKeylessSigningIdentity{
							{
								IssuerMatcher: &workloadidentityv1.SigstoreKeylessSigningIdentity_Issuer{
									Issuer: "https://token.actions.githubusercontent.com/teleport",
								},
								SubjectMatcher: &workloadidentityv1.SigstoreKeylessSigningIdentity_SubjectRegex{
									SubjectRegex: `(.*)/.github/workflows/build.yaml@refs/heads/main`,
								},
							},
						},
						TrustedRoots: []string{githubTrustedRootJSON},
					},
				},
				Requirements: &workloadidentityv1.SigstorePolicyRequirements{
					Attestations: []*workloadidentityv1.InTotoAttestationMatcher{
						{PredicateType: "https://slsa.dev/provenance/v1"},
						{PredicateType: "https://foo.bar.baz"},
					},
				},
			},
			attrs: &workloadidentityv1.WorkloadAttrs{
				Docker: &workloadidentityv1.WorkloadAttrsDocker{
					Container: &workloadidentityv1.WorkloadAttrsDockerContainer{
						ImageDigest: "sha256:c95f6bcc67ae1d482733526520d8bff0e35f4e258a6d0d615c0ed28dd86263e2",
					},
				},
				Sigstore: &workloadidentityv1.WorkloadAttrsSigstore{
					Payloads: mustDecodePayloads(t, githubProvenancePayload),
				},
			},
			error: "No attestation with predicate 'https://foo.bar.baz' found",
		},
		"Failure - Simple Signing Envelope - Signature Mismatch": {
			policy: &workloadidentityv1.SigstorePolicySpec{
				Authority: &workloadidentityv1.SigstorePolicySpec_Key{
					Key: &workloadidentityv1.SigstoreKeyAuthority{
						Public: ecdsaPublicKey,
					},
				},
				Requirements: &workloadidentityv1.SigstorePolicyRequirements{
					ArtifactSignature: true,
				},
			},
			attrs: &workloadidentityv1.WorkloadAttrs{
				Docker: &workloadidentityv1.WorkloadAttrsDocker{
					Container: &workloadidentityv1.WorkloadAttrsDockerContainer{
						ImageDigest: "sha256:aaaa7c9b3350e16ce42c24548347e9078b3562a529df48318a9cc82eafba842c",
						//                   ^^^^
					},
				},
				Sigstore: &workloadidentityv1.WorkloadAttrsSigstore{
					Payloads: func() []*workloadidentityv1.SigstoreVerificationPayload {
						payloads := mustDecodePayloads(t, ecdsaSimpleSigningPayload)

						// Mangle the image digest.
						payloads[0].SimpleSigningEnvelope = bytes.ReplaceAll(
							payloads[0].SimpleSigningEnvelope,
							[]byte(`6f42`),
							[]byte(`aaaa`),
						)
						return payloads
					}(),
				},
			},
			error: "Policy not satisfied by the given bundles",
		},
		"Failure - Simple Signing Envelope - Wrong Image Digest": {
			policy: &workloadidentityv1.SigstorePolicySpec{
				Authority: &workloadidentityv1.SigstorePolicySpec_Key{
					Key: &workloadidentityv1.SigstoreKeyAuthority{
						Public: ecdsaPublicKey,
					},
				},
				Requirements: &workloadidentityv1.SigstorePolicyRequirements{
					ArtifactSignature: true,
				},
			},
			attrs: &workloadidentityv1.WorkloadAttrs{
				Docker: &workloadidentityv1.WorkloadAttrsDocker{
					Container: &workloadidentityv1.WorkloadAttrsDockerContainer{
						ImageDigest: "sha256:c95f6bcc67ae1d482733526520d8bff0e35f4e258a6d0d615c0ed28dd86263e2",
					},
				},
				Sigstore: &workloadidentityv1.WorkloadAttrsSigstore{
					Payloads: mustDecodePayloads(t, ecdsaSimpleSigningPayload),
				},
			},
			error: "Policy not satisfied by the given bundles",
		},
		"Failure - No Payloads": {
			policy: &workloadidentityv1.SigstorePolicySpec{
				Authority: &workloadidentityv1.SigstorePolicySpec_Key{
					Key: &workloadidentityv1.SigstoreKeyAuthority{
						Public: ecdsaPublicKey,
					},
				},
				Requirements: &workloadidentityv1.SigstorePolicyRequirements{
					ArtifactSignature: true,
				},
			},
			attrs: &workloadidentityv1.WorkloadAttrs{
				Docker: &workloadidentityv1.WorkloadAttrsDocker{
					Container: &workloadidentityv1.WorkloadAttrsDockerContainer{
						ImageDigest: "sha256:c95f6bcc67ae1d482733526520d8bff0e35f4e258a6d0d615c0ed28dd86263e2",
					},
				},
				Sigstore: &workloadidentityv1.WorkloadAttrsSigstore{
					Payloads: []*workloadidentityv1.SigstoreVerificationPayload{},
				},
			},
			error: "Policy not satisfied by the given bundles",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			const policyName = "test-policy"

			evaluator, err := sigstore.NewPolicyEvaluator(sigstore.PolicyEvaluatorConfig{
				Store: &testPolicyGetter{
					policies: map[string]*workloadidentityv1.SigstorePolicy{
						policyName: {Spec: tc.policy},
					},
				},
				Logger: logtest.NewLogger(),
			})
			require.NoError(t, err)

			results, err := evaluator.Evaluate(
				context.Background(),
				[]string{policyName},
				&workloadidentityv1.Attrs{Workload: tc.attrs},
			)
			require.NoError(t, err)
			require.Contains(t, results, policyName)

			if tc.error == "" {
				require.NoError(t, results[policyName])
			} else {
				require.ErrorContains(t, results[policyName], tc.error)
			}
		})
	}
}

func mustDecodePayloads(t *testing.T, payloadBytes ...[]byte) []*workloadidentityv1.SigstoreVerificationPayload {
	t.Helper()

	payloads := make([]*workloadidentityv1.SigstoreVerificationPayload, len(payloadBytes))
	for idx, b := range payloadBytes {
		var payload workloadidentityv1.SigstoreVerificationPayload
		require.NoError(t, protojson.UnmarshalOptions{}.Unmarshal(b, &payload))
		payloads[idx] = &payload
	}
	return payloads
}

type testPolicyGetter struct {
	policies map[string]*workloadidentityv1.SigstorePolicy
}

func (g *testPolicyGetter) GetSigstorePolicy(_ context.Context, name string) (*workloadidentityv1.SigstorePolicy, error) {
	if policy, ok := g.policies[name]; ok {
		return policy, nil
	}
	return nil, trace.NotFound("policy not found: %s", name)
}
