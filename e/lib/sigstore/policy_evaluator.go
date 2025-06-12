package sigstore

import (
	"bytes"
	"cmp"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"log/slog"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/gravitational/trace"
	bundlepb "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/sigstore/sigstore/pkg/signature"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport"
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

// TODO(boxofrad): Allow users to configure their own hash algorithm.
const hashAlgorithm = crypto.SHA256

// PolicyEvaluator validates workload signatures and attestations against user
// configured Sigstore policies.
type PolicyEvaluator struct {
	store  PolicyGetter
	logger *slog.Logger
}

// PolicyEvaluatorConfig is the configuration to PolicyEvaluator.
type PolicyEvaluatorConfig struct {
	// Store from which policies will be read.
	Store PolicyGetter

	// Logger that will be used by the evaluator.
	Logger *slog.Logger
}

func (cfg *PolicyEvaluatorConfig) CheckAndSetDefaults() error {
	if cfg.Store == nil {
		return trace.BadParameter("store is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return nil
}

// PolicyGetter contains the parts of lib/services/local.SigstorePolicyService
// we need.
type PolicyGetter interface {
	// GetSigstorePolicy is called to read a policy by name.
	GetSigstorePolicy(ctx context.Context, name string) (*workloadidentityv1.SigstorePolicy, error)
}

// NewPolicyEvaluator created a new PolicyEvaluator with the given configuration.
func NewPolicyEvaluator(cfg PolicyEvaluatorConfig) (*PolicyEvaluator, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, err
	}
	return &PolicyEvaluator{
		store:  cfg.Store,
		logger: cfg.Logger.With(teleport.ComponentKey, "sigstore"),
	}, nil
}

// Evaluate the given workload attributes against the named policies. Returns a
// map where the key is the policy name and the value is the result (error or nil)
// of evaluating the policy.
func (e *PolicyEvaluator) Evaluate(ctx context.Context, policyNames []string, attrs *workloadidentityv1.Attrs) (map[string]error, error) {
	results := make(map[string]error, len(policyNames))

	if len(policyNames) == 0 {
		return results, nil
	}

	digestString := e.imageDigest(attrs)
	if digestString == "" {
		return nil, trace.Errorf("workload attributes include no container image digests")
	}
	digest, err := v1.NewHash(digestString)
	if err != nil {
		return nil, trace.Wrap(err, "parsing container image digest")
	}

	for _, name := range policyNames {
		if _, ok := results[name]; ok {
			continue
		}
		policy, err := e.store.GetSigstorePolicy(ctx, name)
		if err != nil {
			return nil, trace.Wrap(err, "getting Sigstore policy with name %q", name)
		}
		results[name] = e.evaluatePolicy(ctx, policy, digest, attrs.GetWorkload().GetSigstore().GetPayloads())
	}

	return results, nil
}

func (e *PolicyEvaluator) evaluatePolicy(
	ctx context.Context,
	policy *workloadidentityv1.SigstorePolicy,
	digest v1.Hash,
	payloads []*workloadidentityv1.SigstoreVerificationPayload,
) error {
	verifier, identityOpts, err := e.buildVerifier(policy)
	if err != nil {
		return trace.Wrap(err)
	}

	var foundValidSignature bool
	attestedPredicates := make(map[string]struct{})
	errors := make([]error, 0, len(payloads))

	for idx, payload := range payloads {
		var pbBundle bundlepb.Bundle
		if err := proto.Unmarshal(payload.GetBundle(), &pbBundle); err != nil {
			errors = append(errors, trace.Wrap(err, "payload %d, unmarshaling bundle:", idx))
			continue
		}
		bundle, err := bundle.NewBundle(&pbBundle)
		if err != nil {
			errors = append(errors, trace.Wrap(err, "payload %d, validating bundle:", idx))
			continue
		}

		artifactVerifier, err := e.artifactVerifier(policy, digest, payload)
		if err != nil {
			errors = append(errors, trace.Wrap(err, "payload %d:", idx))
			continue
		}
		result, err := verifier.Verify(bundle, verify.NewPolicy(artifactVerifier, identityOpts...))
		if err != nil {
			errors = append(errors, trace.Wrap(err, "payload %d:", idx))
			continue
		}

		foundValidSignature = true
		if pt := result.Statement.GetPredicateType(); pt != "" {
			attestedPredicates[pt] = struct{}{}
		}
	}

	for _, att := range policy.GetSpec().GetRequirements().GetAttestations() {
		if _, ok := attestedPredicates[att.GetPredicateType()]; !ok {
			return trace.AccessDenied("No attestation with predicate '%s' found", att.GetPredicateType())
		}
	}

	if !foundValidSignature {
		const msg = "Policy not satisfied by the given bundles"

		if len(errors) == 0 {
			return trace.Errorf(msg)
		}

		return trace.Wrap(
			trace.NewAggregate(errors...),
			msg,
		)
	}

	return nil
}

func (e *PolicyEvaluator) buildVerifier(policy *workloadidentityv1.SigstorePolicy) (*verify.Verifier, []verify.PolicyOption, error) {
	// TODO(boxofrad): Maybe we should cache these, keyed by a hash of the policy spec.
	var (
		trustedMaterial root.TrustedMaterial
		verifierOpts    []verify.VerifierOption
		identityOpts    []verify.PolicyOption
		err             error
	)
	switch t := policy.GetSpec().GetAuthority().(type) {
	case *workloadidentityv1.SigstorePolicySpec_Key:
		if trustedMaterial, err = e.trustedPublicKeyMaterial(t.Key); err != nil {
			return nil, nil, trace.Wrap(err, "loading trusted public key material")
		}

		// We do not (yet) allow you to use a transparency log or RFC 3161
		// timestamp authority when providing a known public key, so there is
		// no way to obtain the signature's timestamp. We also do not allow you
		// to specify a validity window for a key.
		verifierOpts = append(verifierOpts, verify.WithCurrentTime())

		// When validating against a known public key (rather than a short-lived
		// Fulcio certificate) there is no certificate identity to verify.
		identityOpts = append(identityOpts, verify.WithoutIdentitiesUnsafe())
	case *workloadidentityv1.SigstorePolicySpec_Keyless:
		if trustedMaterial, err = e.keylessTrustedRoots(t.Keyless); err != nil {
			return nil, nil, trace.Wrap(err, "loading trusted public key material")
		}

		for _, id := range t.Keyless.Identities {
			matcher, err := verify.NewShortCertificateIdentity(
				id.GetIssuer(),
				id.GetIssuerRegex(),
				id.GetSubject(),
				id.GetSubjectRegex(),
			)
			if err != nil {
				return nil, nil, trace.Wrap(err, "building certificate identity matcher")
			}
			identityOpts = append(identityOpts, verify.WithCertificateIdentity(matcher))
		}

		if len(trustedMaterial.CTLogs()) != 0 {
			// If there are any transparency logs configured, we require inclusion
			// in at least one of them. In the future, we could make this configurable.
			//
			// Note: if there are no transparency logs or RFC 3161 timestamp
			// authorities configured, WithObserverTimestamps will fail - but
			// you'd need to be blindly trusting the certificates validity so
			// that's probably desirable.
			verifierOpts = append(verifierOpts, verify.WithTransparencyLog(1))
		}

		verifierOpts = append(verifierOpts, verify.WithObserverTimestamps(1))
	default:
		return nil, nil, trace.BadParameter("key or keyless authority is required")
	}

	verifier, err := verify.NewVerifier(trustedMaterial, verifierOpts...)
	if err != nil {
		return nil, nil, trace.Wrap(err, "creating signed entity verifier")
	}
	return verifier, identityOpts, nil
}

func (e *PolicyEvaluator) trustedPublicKeyMaterial(key *workloadidentityv1.SigstoreKeyAuthority) (*root.TrustedPublicKeyMaterial, error) {
	block, _ := pem.Decode([]byte(key.GetPublic()))
	if block == nil {
		return nil, trace.Errorf("failed to decode PEM")
	}

	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, trace.Wrap(err, "parsing PKIX public key")
	}

	var verifier signature.Verifier
	switch k := pubKey.(type) {
	case *rsa.PublicKey:
		// TODO(boxofrad): Cosign only supports RSA PKCS#1.5 padded keys, but
		// sigstore-go supports RSA PSS so we should either "fall back" between
		// algorithms or allow the user to choose their algorithm.
		if verifier, err = signature.LoadRSAPKCS1v15Verifier(k, hashAlgorithm); err != nil {
			return nil, trace.Wrap(err, "loading RSA verifier")
		}
	case *ecdsa.PublicKey:
		if verifier, err = signature.LoadECDSAVerifier(k, hashAlgorithm); err != nil {
			return nil, trace.Wrap(err, "loading ECDSA verifier")
		}
	case ed25519.PublicKey:
		if verifier, err = signature.LoadED25519Verifier(k); err != nil {
			return nil, trace.Wrap(err, "loading ED25519 verifier")
		}
	default:
		return nil, trace.Errorf("unsupported public key type: %T", pubKey)
	}

	return root.NewTrustedPublicKeyMaterial(func(string) (root.TimeConstrainedVerifier, error) {
		return nonExpiringVerifier{verifier}, nil
	}), nil
}

// TODO(boxofrad): Allow users to supply a list of public keys with different
// validity periods.
type nonExpiringVerifier struct{ signature.Verifier }

func (nonExpiringVerifier) ValidAtTime(time.Time) bool { return true }

func (e *PolicyEvaluator) keylessTrustedRoots(keyless *workloadidentityv1.SigstoreKeylessAuthority) (root.TrustedMaterial, error) {
	// No custom trusted roots, use TUF to get the Public Good instance.
	if len(keyless.GetTrustedRoots()) == 0 {
		client, err := tuf.DefaultClient()
		if err != nil {
			return nil, trace.Wrap(err, "getting TUF client")
		}
		trustedRootJSON, err := client.GetTarget("trusted_root.json")
		if err != nil {
			return nil, trace.Wrap(err, "getting trusted_root.json target")
		}
		trustedRoot, err := root.NewTrustedRootFromJSON(trustedRootJSON)
		if err != nil {
			return nil, trace.Wrap(err, "parsing trusted root")
		}
		return trustedRoot, nil
	}

	roots := make(root.TrustedMaterialCollection, len(keyless.GetTrustedRoots()))
	for idx, trustedRootJSON := range keyless.GetTrustedRoots() {
		trustedRoot, err := root.NewTrustedRootFromJSON([]byte(trustedRootJSON))
		if err != nil {
			return nil, trace.Wrap(err, "parsing trusted root [%d]", idx)
		}
		roots[idx] = trustedRoot
	}
	return roots, nil
}

func (e *PolicyEvaluator) imageDigest(attrs *workloadidentityv1.Attrs) string {
	// It's technically possible for `tbot` to set more than one of these, but
	// unlikely because the attestors rely on Kubernetes, Docker, and Podman
	// using quite different cgroup names.
	return cmp.Or(
		attrs.GetWorkload().GetKubernetes().GetContainer().GetImageDigest(),
		attrs.GetWorkload().GetPodman().GetContainer().GetImageDigest(),
		attrs.GetWorkload().GetDocker().GetContainer().GetImageDigest(),
	)
}

func (e *PolicyEvaluator) artifactVerifier(
	policy *workloadidentityv1.SigstorePolicy,
	imageDigest v1.Hash,
	payload *workloadidentityv1.SigstoreVerificationPayload,
) (verify.ArtifactPolicyOption, error) {
	requirements := policy.GetSpec().GetRequirements()

	// These should be caught when the SigstorePolicy is created, but it's best
	// to be explicit here rather than return a confusing error at verification
	// time if a misconfiguration does get through.
	//
	// We broadly support two kinds of signatures:
	//
	// 	1. Artifact signatures created by cosign. These are signatures over the
	// 	   hash of a Simple Signing Envelope containing the artifact digest.
	//
	// 	2. In-toto attestations, like the ones created by GitHub. Where the
	// 	   artifact digest is encoded in the attestation itself (i.e. no envelope).
	//
	// We treat them as mutually-exclusive because they're handled differently
	// in sigstore-go. While we *could* do something smarter, it's actually
	// quite unlikely an artifact has both kinds of signatures. Instead, you can
	// use a custom attestation *as a traditional artifact signature* which is
	// what the Sigstore maintainers are planning to do with cosign eventually.
	//
	// Alternatively, if you truly need both, you can always create two policies.
	if len(requirements.GetAttestations()) == 0 && !requirements.GetArtifactSignature() {
		return nil, trace.BadParameter("cannot evaluate a policy with no requirements")
	}
	if len(requirements.GetAttestations()) != 0 && requirements.GetArtifactSignature() {
		return nil, trace.BadParameter("requirements.artifact_signature and requirements.attestations are mutually-exclusive")
	}

	if len(requirements.GetAttestations()) != 0 {
		hash, err := hex.DecodeString(imageDigest.Hex)
		if err != nil {
			return nil, trace.Wrap(err, "hex-decoding image digest")
		}
		return verify.WithArtifactDigest(imageDigest.Algorithm, hash), nil
	}

	if payload.GetSimpleSigningEnvelope() == nil {
		return nil, trace.AccessDenied("requirements.artifact_signature requires the verification payload to include a simple signing envelope")
	}

	var envelope struct {
		Critical struct {
			Image struct {
				Digest string `json:"docker-manifest-digest"` // This is used for all OCI images, not just Docker.
			} `json:"image"`
		} `json:"critical"`
	}
	if err := json.Unmarshal(payload.GetSimpleSigningEnvelope(), &envelope); err != nil {
		return nil, trace.Wrap(err, "unmarshaling simple signing envelope")
	}
	if envelope.Critical.Image.Digest != imageDigest.String() {
		return nil, trace.AccessDenied(
			"simple signing envelope `critical.image.docker-manifest-digest` (%s) does not match expected image digest (%s)",
			envelope.Critical.Image.Digest,
			imageDigest.String(),
		)
	}
	return verify.WithArtifact(bytes.NewReader(payload.GetSimpleSigningEnvelope())), nil
}
