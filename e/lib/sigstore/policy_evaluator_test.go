package sigstore_test

import (
	"bytes"
	"context"
	_ "embed"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

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
			policy: workloadidentityv1.SigstorePolicySpec_builder{
				Key: workloadidentityv1.SigstoreKeyAuthority_builder{
					Public: ecdsaPublicKey,
				}.Build(),
				Requirements: workloadidentityv1.SigstorePolicyRequirements_builder{
					ArtifactSignature: true,
				}.Build(),
			}.Build(),
			attrs: workloadidentityv1.WorkloadAttrs_builder{
				Docker: workloadidentityv1.WorkloadAttrsDocker_builder{
					Container: workloadidentityv1.WorkloadAttrsDockerContainer_builder{
						ImageDigest: "sha256:6f427c9b3350e16ce42c24548347e9078b3562a529df48318a9cc82eafba842c",
					}.Build(),
				}.Build(),
				Sigstore: workloadidentityv1.WorkloadAttrsSigstore_builder{
					Payloads: mustDecodePayloads(t, ecdsaSimpleSigningPayload),
				}.Build(),
			}.Build(),
		},
		"Success - RSA Keypair - Simple Signing Envelope": {
			policy: workloadidentityv1.SigstorePolicySpec_builder{
				Key: workloadidentityv1.SigstoreKeyAuthority_builder{
					Public: rsaPublicKey,
				}.Build(),
				Requirements: workloadidentityv1.SigstorePolicyRequirements_builder{
					ArtifactSignature: true,
				}.Build(),
			}.Build(),
			attrs: workloadidentityv1.WorkloadAttrs_builder{
				Docker: workloadidentityv1.WorkloadAttrsDocker_builder{
					Container: workloadidentityv1.WorkloadAttrsDockerContainer_builder{
						ImageDigest: "sha256:6f427c9b3350e16ce42c24548347e9078b3562a529df48318a9cc82eafba842c",
					}.Build(),
				}.Build(),
				Sigstore: workloadidentityv1.WorkloadAttrsSigstore_builder{
					Payloads: mustDecodePayloads(t, rsaSimpleSigningPayload),
				}.Build(),
			}.Build(),
		},
		"Success - ED25519 Keypair - Simple Signing Envelope": {
			policy: workloadidentityv1.SigstorePolicySpec_builder{
				Key: workloadidentityv1.SigstoreKeyAuthority_builder{
					Public: ed25519PublicKey,
				}.Build(),
				Requirements: workloadidentityv1.SigstorePolicyRequirements_builder{
					ArtifactSignature: true,
				}.Build(),
			}.Build(),
			attrs: workloadidentityv1.WorkloadAttrs_builder{
				Docker: workloadidentityv1.WorkloadAttrsDocker_builder{
					Container: workloadidentityv1.WorkloadAttrsDockerContainer_builder{
						ImageDigest: "sha256:6f427c9b3350e16ce42c24548347e9078b3562a529df48318a9cc82eafba842c",
					}.Build(),
				}.Build(),
				Sigstore: workloadidentityv1.WorkloadAttrsSigstore_builder{
					Payloads: mustDecodePayloads(t, ed25519SimpleSigningPayload),
				}.Build(),
			}.Build(),
		},
		"Failure - Key Mismatch": {
			policy: workloadidentityv1.SigstorePolicySpec_builder{
				Key: workloadidentityv1.SigstoreKeyAuthority_builder{
					Public: ecdsaPublicKey,
				}.Build(),
				Requirements: workloadidentityv1.SigstorePolicyRequirements_builder{
					ArtifactSignature: true,
				}.Build(),
			}.Build(),
			attrs: workloadidentityv1.WorkloadAttrs_builder{
				Docker: workloadidentityv1.WorkloadAttrsDocker_builder{
					Container: workloadidentityv1.WorkloadAttrsDockerContainer_builder{
						ImageDigest: "sha256:6f427c9b3350e16ce42c24548347e9078b3562a529df48318a9cc82eafba842c",
					}.Build(),
				}.Build(),
				Sigstore: workloadidentityv1.WorkloadAttrsSigstore_builder{
					Payloads: mustDecodePayloads(t, rsaSimpleSigningPayload), // <- Signed by a different key
				}.Build(),
			}.Build(),
			error: "Policy not satisfied by the given bundles",
		},
		"Success - Keyless Public Good - Simple Signing Envelope": {
			policy: workloadidentityv1.SigstorePolicySpec_builder{
				Keyless: workloadidentityv1.SigstoreKeylessAuthority_builder{
					Identities: []*workloadidentityv1.SigstoreKeylessSigningIdentity{
						workloadidentityv1.SigstoreKeylessSigningIdentity_builder{
							Issuer:  proto.String("https://accounts.google.com"),
							Subject: proto.String("daniel.upton@goteleport.com"),
						}.Build(),
					},
				}.Build(),
				Requirements: workloadidentityv1.SigstorePolicyRequirements_builder{
					ArtifactSignature: true,
				}.Build(),
			}.Build(),
			attrs: workloadidentityv1.WorkloadAttrs_builder{
				Docker: workloadidentityv1.WorkloadAttrsDocker_builder{
					Container: workloadidentityv1.WorkloadAttrsDockerContainer_builder{
						ImageDigest: "sha256:6f427c9b3350e16ce42c24548347e9078b3562a529df48318a9cc82eafba842c",
					}.Build(),
				}.Build(),
				Sigstore: workloadidentityv1.WorkloadAttrsSigstore_builder{
					Payloads: mustDecodePayloads(t, keylessSimpleSigningPayload),
				}.Build(),
			}.Build(),
		},
		"Success - Keyless Custom Trusted Root - Attestations": {
			policy: workloadidentityv1.SigstorePolicySpec_builder{
				Keyless: workloadidentityv1.SigstoreKeylessAuthority_builder{
					Identities: []*workloadidentityv1.SigstoreKeylessSigningIdentity{
						workloadidentityv1.SigstoreKeylessSigningIdentity_builder{
							Issuer:       proto.String("https://token.actions.githubusercontent.com/teleport"),
							SubjectRegex: proto.String(`(.*)/.github/workflows/build.yaml@refs/heads/main`),
						}.Build(),
					},
					TrustedRoots: []string{githubTrustedRootJSON},
				}.Build(),
				Requirements: workloadidentityv1.SigstorePolicyRequirements_builder{
					Attestations: []*workloadidentityv1.InTotoAttestationMatcher{
						workloadidentityv1.InTotoAttestationMatcher_builder{PredicateType: "https://slsa.dev/provenance/v1"}.Build(),
					},
				}.Build(),
			}.Build(),
			attrs: workloadidentityv1.WorkloadAttrs_builder{
				Docker: workloadidentityv1.WorkloadAttrsDocker_builder{
					Container: workloadidentityv1.WorkloadAttrsDockerContainer_builder{
						ImageDigest: "sha256:c95f6bcc67ae1d482733526520d8bff0e35f4e258a6d0d615c0ed28dd86263e2",
					}.Build(),
				}.Build(),
				Sigstore: workloadidentityv1.WorkloadAttrsSigstore_builder{
					Payloads: mustDecodePayloads(t, githubProvenancePayload),
				}.Build(),
			}.Build(),
		},
		"Failure - Missing Attestation": {
			policy: workloadidentityv1.SigstorePolicySpec_builder{
				Keyless: workloadidentityv1.SigstoreKeylessAuthority_builder{
					Identities: []*workloadidentityv1.SigstoreKeylessSigningIdentity{
						workloadidentityv1.SigstoreKeylessSigningIdentity_builder{
							Issuer:       proto.String("https://token.actions.githubusercontent.com/teleport"),
							SubjectRegex: proto.String(`(.*)/.github/workflows/build.yaml@refs/heads/main`),
						}.Build(),
					},
					TrustedRoots: []string{githubTrustedRootJSON},
				}.Build(),
				Requirements: workloadidentityv1.SigstorePolicyRequirements_builder{
					Attestations: []*workloadidentityv1.InTotoAttestationMatcher{
						workloadidentityv1.InTotoAttestationMatcher_builder{PredicateType: "https://slsa.dev/provenance/v1"}.Build(),
						workloadidentityv1.InTotoAttestationMatcher_builder{PredicateType: "https://foo.bar.baz"}.Build(),
					},
				}.Build(),
			}.Build(),
			attrs: workloadidentityv1.WorkloadAttrs_builder{
				Docker: workloadidentityv1.WorkloadAttrsDocker_builder{
					Container: workloadidentityv1.WorkloadAttrsDockerContainer_builder{
						ImageDigest: "sha256:c95f6bcc67ae1d482733526520d8bff0e35f4e258a6d0d615c0ed28dd86263e2",
					}.Build(),
				}.Build(),
				Sigstore: workloadidentityv1.WorkloadAttrsSigstore_builder{
					Payloads: mustDecodePayloads(t, githubProvenancePayload),
				}.Build(),
			}.Build(),
			error: "No attestation with predicate 'https://foo.bar.baz' found",
		},
		"Failure - Simple Signing Envelope - Signature Mismatch": {
			policy: workloadidentityv1.SigstorePolicySpec_builder{
				Key: workloadidentityv1.SigstoreKeyAuthority_builder{
					Public: ecdsaPublicKey,
				}.Build(),
				Requirements: workloadidentityv1.SigstorePolicyRequirements_builder{
					ArtifactSignature: true,
				}.Build(),
			}.Build(),
			attrs: workloadidentityv1.WorkloadAttrs_builder{
				Docker: workloadidentityv1.WorkloadAttrsDocker_builder{
					Container: workloadidentityv1.WorkloadAttrsDockerContainer_builder{
						ImageDigest: "sha256:aaaa7c9b3350e16ce42c24548347e9078b3562a529df48318a9cc82eafba842c",
						//                   ^^^^
					}.Build(),
				}.Build(),
				Sigstore: workloadidentityv1.WorkloadAttrsSigstore_builder{
					Payloads: func() []*workloadidentityv1.SigstoreVerificationPayload {
						payloads := mustDecodePayloads(t, ecdsaSimpleSigningPayload)

						// Mangle the image digest.
						if x := bytes.ReplaceAll(
							payloads[0].GetSimpleSigningEnvelope(),
							[]byte(`6f42`),
							[]byte(`aaaa`),
						); x != nil {
							payloads[0].SetSimpleSigningEnvelope(x)
						} else {
							payloads[0].ClearSimpleSigningEnvelope()
						}
						return payloads
					}(),
				}.Build(),
			}.Build(),
			error: "Policy not satisfied by the given bundles",
		},
		"Failure - Simple Signing Envelope - Wrong Image Digest": {
			policy: workloadidentityv1.SigstorePolicySpec_builder{
				Key: workloadidentityv1.SigstoreKeyAuthority_builder{
					Public: ecdsaPublicKey,
				}.Build(),
				Requirements: workloadidentityv1.SigstorePolicyRequirements_builder{
					ArtifactSignature: true,
				}.Build(),
			}.Build(),
			attrs: workloadidentityv1.WorkloadAttrs_builder{
				Docker: workloadidentityv1.WorkloadAttrsDocker_builder{
					Container: workloadidentityv1.WorkloadAttrsDockerContainer_builder{
						ImageDigest: "sha256:c95f6bcc67ae1d482733526520d8bff0e35f4e258a6d0d615c0ed28dd86263e2",
					}.Build(),
				}.Build(),
				Sigstore: workloadidentityv1.WorkloadAttrsSigstore_builder{
					Payloads: mustDecodePayloads(t, ecdsaSimpleSigningPayload),
				}.Build(),
			}.Build(),
			error: "Policy not satisfied by the given bundles",
		},
		"Failure - No Payloads": {
			policy: workloadidentityv1.SigstorePolicySpec_builder{
				Key: workloadidentityv1.SigstoreKeyAuthority_builder{
					Public: ecdsaPublicKey,
				}.Build(),
				Requirements: workloadidentityv1.SigstorePolicyRequirements_builder{
					ArtifactSignature: true,
				}.Build(),
			}.Build(),
			attrs: workloadidentityv1.WorkloadAttrs_builder{
				Docker: workloadidentityv1.WorkloadAttrsDocker_builder{
					Container: workloadidentityv1.WorkloadAttrsDockerContainer_builder{
						ImageDigest: "sha256:c95f6bcc67ae1d482733526520d8bff0e35f4e258a6d0d615c0ed28dd86263e2",
					}.Build(),
				}.Build(),
				Sigstore: workloadidentityv1.WorkloadAttrsSigstore_builder{
					Payloads: []*workloadidentityv1.SigstoreVerificationPayload{},
				}.Build(),
			}.Build(),
			error: "Policy not satisfied by the given bundles",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			const policyName = "test-policy"

			evaluator, err := sigstore.NewPolicyEvaluator(sigstore.PolicyEvaluatorConfig{
				Store: &testPolicyGetter{
					policies: map[string]*workloadidentityv1.SigstorePolicy{
						policyName: workloadidentityv1.SigstorePolicy_builder{Spec: tc.policy}.Build(),
					},
				},
				Logger: logtest.NewLogger(),
			})
			require.NoError(t, err)

			results, err := evaluator.Evaluate(
				context.Background(),
				[]string{policyName},
				workloadidentityv1.Attrs_builder{Workload: tc.attrs}.Build(),
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
