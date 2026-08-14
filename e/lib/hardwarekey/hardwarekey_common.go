package hardwarekey

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"time"

	"github.com/gravitational/trace"

	attestation "github.com/gravitational/teleport/api/gen/proto/go/attestation/v1"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

// AttestationServer is used to store and retrieve attestation data in the backend.
type AttestationServer interface {
	// UpsertKeyAttestationData upserts a public key's verified attestation data.
	UpsertKeyAttestationData(ctx context.Context, attestationData *keys.AttestationData, ttl time.Duration) error
	// GetKeyAttestationData gets a public key's verified attestation data.
	GetKeyAttestationData(ctx context.Context, pubDER []byte) (*keys.AttestationData, error)
}

// AttestHardwareKey attests a hardware key, either with the given statement or a
// previously stored attestation data matching the given public key.
func AttestHardwareKey(ctx context.Context, server AttestationServer, attestation *hardwarekey.AttestationStatement, pub crypto.PublicKey, sessionTTL time.Duration) (*keys.AttestationData, error) {
	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// No attestation statement provided. This means either:
	//   1. This is a reissue request which uses the cached attestation response stored at login time.
	//   2. This is a login request with a non-hardware private key, which should result in an error.
	if attestation == nil {
		attestationData, err := server.GetKeyAttestationData(ctx, pubDER)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return attestationData, nil
	}

	// Using the given attestation statement, get the attestation data
	// of the given public key.
	attestationData, err := attestHardwareKey(attestation)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Verify that the given public key matches the attestation statement.
	if !bytes.Equal(pubDER, attestationData.PublicKeyDER) {
		return nil, trace.BadParameter("the provided attestation statement does not match the given public key")
	}

	if err := server.UpsertKeyAttestationData(ctx, attestationData, sessionTTL); err != nil {
		return nil, trace.Wrap(err)
	}

	return attestationData, nil
}

// attestHardwareKey performs attestation using the given attestation statement,
// and returns verified attestation data.
func attestHardwareKey(att *hardwarekey.AttestationStatement) (*keys.AttestationData, error) {
	protoReq := att.ToProto()
	switch protoReq.GetAttestationStatement().(type) {
	case *attestation.AttestationStatement_YubikeyAttestationStatement:
		return attestYubikey(protoReq.GetYubikeyAttestationStatement())
	default:
		return nil, trace.BadParameter("unexpected attestation statement type %T", protoReq.GetAttestationStatement())
	}
}
