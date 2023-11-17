package hardwarekey

import (
	"context"
	"crypto"
	"time"

	"github.com/gravitational/trace"

	attestation "github.com/gravitational/teleport/api/gen/proto/go/attestation/v1"
	"github.com/gravitational/teleport/api/utils/keys"
)

// AttestationServer is used to store and retrieve attestation data in the backend.
type AttestationServer interface {
	// UpsertKeyAttestationData upserts a public key's verified attestation data.
	UpsertKeyAttestationData(ctx context.Context, attestationData *keys.AttestationData, ttl time.Duration) error
	// GetKeyAttestationData gets a public key's verified attestation data.
	GetKeyAttestationData(ctx context.Context, publicKey crypto.PublicKey) (*keys.AttestationData, error)
}

// AttestHardwareKey attests a hardware key, either with the given statement or a
// previously stored attestation data matching the given public key.
func AttestHardwareKey(ctx context.Context, server AttestationServer, requiredKeyPolicy keys.PrivateKeyPolicy, att *keys.AttestationStatement, pub crypto.PublicKey, sessionTTL time.Duration) (keys.PrivateKeyPolicy, error) {
	// Get the private key policy met by the given public key. If attestation statement
	// is not given, then no private key policy will be met.
	privateKeyPolicy := keys.PrivateKeyPolicyNone
	if att != nil {
		attData, err := attestHardwareKey(att)
		if err != nil {
			return "", trace.Wrap(err)
		}
		privateKeyPolicy = attData.PrivateKeyPolicy
		if err := server.UpsertKeyAttestationData(ctx, attData, sessionTTL); err != nil {
			return "", trace.Wrap(err)
		}
	} else {
		// No attestation statement provided. This means either:
		//   1. This is a reissue request which uses the cached attestation response stored at login time.
		//   2. This is a login request with a non-hardware private key.
		// In both cases, we can check for a cached attestation response to decide whether or not to proceed.
		attData, err := server.GetKeyAttestationData(ctx, pub)
		if err != nil {
			if !trace.IsNotFound(err) {
				return "", trace.Wrap(err)
			}
		} else {
			privateKeyPolicy = attData.PrivateKeyPolicy
		}
	}

	// Check that the attested private key policy is sufficient for the required private key policy.
	if err := requiredKeyPolicy.VerifyPolicy(privateKeyPolicy); err != nil {
		return "", trace.Wrap(err)
	}

	return privateKeyPolicy, nil
}

// attestHardwareKey performs attestation using the given attestation statement,
// and returns verified attestation data.
func attestHardwareKey(att *keys.AttestationStatement) (*keys.AttestationData, error) {
	protoReq := att.ToProto()
	switch protoReq.GetAttestationStatement().(type) {
	case *attestation.AttestationStatement_YubikeyAttestationStatement:
		return attestYubikey(protoReq.GetYubikeyAttestationStatement())
	default:
		return nil, trace.BadParameter("unexpected attestation statement type %T", protoReq.GetAttestationStatement())
	}
}
