/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package tpm

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
