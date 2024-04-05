package tpmjoin

import (
	"github.com/google/go-attestation/attest"

	"github.com/gravitational/teleport/api/client/proto"
)

// AttestationParametersToProto converts an attest.AttestationParameters to
// its protobuf representation.
func AttestationParametersToProto(in attest.AttestationParameters) *proto.TPMAttestationParameters {
	return &proto.TPMAttestationParameters{
		Public:            in.Public,
		CreateData:        in.CreateData,
		CreateAttestation: in.CreateAttestation,
		CreateSignature:   in.CreateSignature,
	}
}

// AttestationParametersFromProto extracts an attest.AttestationParameters from
// its protobuf representation.
func AttestationParametersFromProto(in *proto.TPMAttestationParameters) attest.AttestationParameters {
	if in == nil {
		return attest.AttestationParameters{}
	}
	return attest.AttestationParameters{
		Public:            in.Public,
		CreateData:        in.CreateData,
		CreateAttestation: in.CreateAttestation,
		CreateSignature:   in.CreateSignature,
	}
}

// EncryptedCredentialToProto converts an attest.EncryptedCredential to
// its protobuf representation.
func EncryptedCredentialToProto(in *attest.EncryptedCredential) *proto.TPMEncryptedCredential {
	if in == nil {
		return nil
	}
	return &proto.TPMEncryptedCredential{
		CredentialBlob: in.Credential,
		Secret:         in.Secret,
	}
}

// EncryptedCredentialFromProto extracts an attest.EncryptedCredential from
// its protobuf representation.
func EncryptedCredentialFromProto(in *proto.TPMEncryptedCredential) *attest.EncryptedCredential {
	if in == nil {
		return nil
	}
	return &attest.EncryptedCredential{
		Credential: in.CredentialBlob,
		Secret:     in.Secret,
	}
}
