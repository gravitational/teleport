/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package devicetrust

import (
	"crypto"

	"github.com/google/go-attestation/attest"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// AttestationParametersToProto converts an attest.AttestationParameters to
// its protobuf representation.
func AttestationParametersToProto(in attest.AttestationParameters) *devicepb.TPMAttestationParameters {
	return &devicepb.TPMAttestationParameters{
		Public:            in.Public,
		CreateData:        in.CreateData,
		CreateAttestation: in.CreateAttestation,
		CreateSignature:   in.CreateSignature,
	}
}

// AttestationParametersFromProto extracts an attest.AttestationParameters from
// its protobuf representation.
func AttestationParametersFromProto(in *devicepb.TPMAttestationParameters) attest.AttestationParameters {
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
func EncryptedCredentialToProto(in *attest.EncryptedCredential) *devicepb.TPMEncryptedCredential {
	if in == nil {
		return nil
	}
	return &devicepb.TPMEncryptedCredential{
		CredentialBlob: in.Credential,
		Secret:         in.Secret,
	}
}

// EncryptedCredentialFromProto extracts an attest.EncryptedCredential from
// its protobuf representation.
func EncryptedCredentialFromProto(in *devicepb.TPMEncryptedCredential) *attest.EncryptedCredential {
	if in == nil {
		return nil
	}
	return &attest.EncryptedCredential{
		Credential: in.CredentialBlob,
		Secret:     in.Secret,
	}
}

// PlatformParametersToProto converts an attest.PlatformParameters to
// its protobuf representation.
func PlatformParametersToProto(in *attest.PlatformParameters) *devicepb.TPMPlatformParameters {
	if in == nil {
		return nil
	}
	return &devicepb.TPMPlatformParameters{
		EventLog: in.EventLog,
		Quotes:   quotesToProto(in.Quotes),
		Pcrs:     pcrsToProto(in.PCRs),
	}
}

// PlatformParametersFromProto extracts an attest.PlatformParameters from
// its protobuf representation.
func PlatformParametersFromProto(in *devicepb.TPMPlatformParameters) *attest.PlatformParameters {
	if in == nil {
		return nil
	}
	return &attest.PlatformParameters{
		TPMVersion: attest.TPMVersion20,
		Quotes:     quotesFromProto(in.Quotes),
		PCRs:       pcrsFromProto(in.Pcrs),
		EventLog:   in.EventLog,
	}
}

// PlatformAttestationToProto converts an *attest.PlatformParameters and nonce
// to a PlatformAttestation proto message.
func PlatformAttestationToProto(in *attest.PlatformParameters, nonce []byte) *devicepb.TPMPlatformAttestation {
	if in == nil {
		return nil
	}
	platParams := PlatformParametersToProto(in)
	return &devicepb.TPMPlatformAttestation{
		PlatformParameters: platParams,
		Nonce:              nonce,
	}
}

// PlatformAttestationFromProto extracts a attest.PlatformParameters and nonce
// from a PlatformAttestation proto message.
func PlatformAttestationFromProto(in *devicepb.TPMPlatformAttestation) (platParams *attest.PlatformParameters, nonce []byte) {
	if in == nil {
		return nil, nil
	}
	return PlatformParametersFromProto(in.PlatformParameters), in.Nonce
}

func quotesToProto(in []attest.Quote) []*devicepb.TPMQuote {
	out := make([]*devicepb.TPMQuote, len(in))
	for i, q := range in {
		out[i] = &devicepb.TPMQuote{
			Quote:     q.Quote,
			Signature: q.Signature,
		}
	}
	return out
}

func quotesFromProto(in []*devicepb.TPMQuote) []attest.Quote {
	out := make([]attest.Quote, len(in))
	for i, q := range in {
		out[i] = attest.Quote{
			Version:   attest.TPMVersion20,
			Quote:     q.Quote,
			Signature: q.Signature,
		}
	}
	return out
}

func pcrsToProto(in []attest.PCR) []*devicepb.TPMPCR {
	out := make([]*devicepb.TPMPCR, len(in))
	for i, pcr := range in {
		out[i] = &devicepb.TPMPCR{
			Index:     int32(pcr.Index),
			Digest:    pcr.Digest,
			DigestAlg: uint64(pcr.DigestAlg),
		}
	}
	return out
}

func pcrsFromProto(in []*devicepb.TPMPCR) []attest.PCR {
	out := make([]attest.PCR, len(in))
	for i, pcr := range in {
		out[i] = attest.PCR{
			Index:     int(pcr.Index),
			Digest:    pcr.Digest,
			DigestAlg: crypto.Hash(pcr.DigestAlg),
		}
	}
	return out
}
