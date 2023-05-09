package devicetrust

import (
	"crypto"
	"github.com/google/go-attestation/attest"
	"github.com/gravitational/teleport/api/utils"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAttestationParametersProto(t *testing.T) {
	want := attest.AttestationParameters{
		Public:            []byte("public"),
		CreateData:        []byte("create_data"),
		CreateAttestation: []byte("create_attestation"),
		CreateSignature:   []byte("create_signature"),
	}
	pb := AttestationParametersToProto(want)
	clonedPb := utils.CloneProtoMsg(pb)
	got := AttestationParametersFromProto(clonedPb)
	require.Equal(t, want, got)
}

func TestEncryptedCredentialProto(t *testing.T) {
	want := attest.EncryptedCredential{
		Credential: []byte("encrypted_credential"),
		Secret:     []byte("secret"),
	}
	pb := EncryptedCredentialToProto(&want)
	clonedPb := utils.CloneProtoMsg(pb)
	got := EncryptedCredentialFromProto(clonedPb)
	require.Equal(t, want, got)
}

func TestPlatformParametersProto(t *testing.T) {
	want := attest.PlatformParameters{
		TPMVersion: attest.TPMVersion20,
		EventLog:   []byte("event_log"),
		Public:     []byte("public"),
		Quotes: []attest.Quote{
			{
				Version:   attest.TPMVersion20,
				Quote:     []byte("quote_0"),
				Signature: []byte("signature_0"),
			},
			{
				Version:   attest.TPMVersion20,
				Quote:     []byte("quote_1"),
				Signature: []byte("signature_1"),
			},
		},
		PCRs: []attest.PCR{
			{
				Index:     0,
				Digest:    []byte("digest_0"),
				DigestAlg: crypto.SHA256,
			},
			{
				Index:     0,
				Digest:    []byte("digest_1"),
				DigestAlg: crypto.SHA256,
			},
		},
	}
	pb := PlatformParametersToProto(&want)
	clonedPb := utils.CloneProtoMsg(pb)
	got := PlatformParametersFromProto(clonedPb)
	// We expect `Public` to be nil because we don't transmit this field over
	// the wire. This is because we don't use this value and rely on our stored
	// version of the key.
	want.Public = nil
	require.Equal(t, want, got)
}
