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
	return devicepb.TPMAttestationParameters_builder{
		Public:            in.Public,
		CreateData:        in.CreateData,
		CreateAttestation: in.CreateAttestation,
		CreateSignature:   in.CreateSignature,
	}.Build()
}

// AttestationParametersFromProto extracts an attest.AttestationParameters from
// its protobuf representation.
func AttestationParametersFromProto(in *devicepb.TPMAttestationParameters) attest.AttestationParameters {
	if in == nil {
		return attest.AttestationParameters{}
	}
	return attest.AttestationParameters{
		Public:            in.GetPublic(),
		CreateData:        in.GetCreateData(),
		CreateAttestation: in.GetCreateAttestation(),
		CreateSignature:   in.GetCreateSignature(),
	}
}

// EncryptedCredentialToProto converts an attest.EncryptedCredential to
// its protobuf representation.
func EncryptedCredentialToProto(in *attest.EncryptedCredential) *devicepb.TPMEncryptedCredential {
	if in == nil {
		return nil
	}
	return devicepb.TPMEncryptedCredential_builder{
		CredentialBlob: in.Credential,
		Secret:         in.Secret,
	}.Build()
}

// EncryptedCredentialFromProto extracts an attest.EncryptedCredential from
// its protobuf representation.
func EncryptedCredentialFromProto(in *devicepb.TPMEncryptedCredential) *attest.EncryptedCredential {
	if in == nil {
		return nil
	}
	return &attest.EncryptedCredential{
		Credential: in.GetCredentialBlob(),
		Secret:     in.GetSecret(),
	}
}

// PlatformParametersToProto converts an attest.PlatformParameters to
// its protobuf representation.
func PlatformParametersToProto(in *attest.PlatformParameters) *devicepb.TPMPlatformParameters {
	if in == nil {
		return nil
	}
	return devicepb.TPMPlatformParameters_builder{
		EventLog: in.EventLog,
		Quotes:   quotesToProto(in.Quotes),
		Pcrs:     pcrsToProto(in.PCRs),
	}.Build()
}

// PlatformParametersFromProto extracts an attest.PlatformParameters from
// its protobuf representation.
func PlatformParametersFromProto(in *devicepb.TPMPlatformParameters) *attest.PlatformParameters {
	if in == nil {
		return nil
	}
	return &attest.PlatformParameters{
		Quotes:   quotesFromProto(in.GetQuotes()),
		PCRs:     pcrsFromProto(in.GetPcrs()),
		EventLog: in.GetEventLog(),
	}
}

// PlatformAttestationToProto converts an *attest.PlatformParameters and nonce
// to a PlatformAttestation proto message.
func PlatformAttestationToProto(in *attest.PlatformParameters, nonce []byte) *devicepb.TPMPlatformAttestation {
	if in == nil {
		return nil
	}
	platParams := PlatformParametersToProto(in)
	return devicepb.TPMPlatformAttestation_builder{
		PlatformParameters: platParams,
		Nonce:              nonce,
	}.Build()
}

// PlatformAttestationFromProto extracts a attest.PlatformParameters and nonce
// from a PlatformAttestation proto message.
func PlatformAttestationFromProto(in *devicepb.TPMPlatformAttestation) (platParams *attest.PlatformParameters, nonce []byte) {
	if in == nil {
		return nil, nil
	}
	return PlatformParametersFromProto(in.GetPlatformParameters()), in.GetNonce()
}

func quotesToProto(in []attest.Quote) []*devicepb.TPMQuote {
	out := make([]*devicepb.TPMQuote, len(in))
	for i, q := range in {
		out[i] = devicepb.TPMQuote_builder{
			Quote:     q.Quote,
			Signature: q.Signature,
		}.Build()
	}
	return out
}

func quotesFromProto(in []*devicepb.TPMQuote) []attest.Quote {
	out := make([]attest.Quote, len(in))
	for i, q := range in {
		out[i] = attest.Quote{
			Quote:     q.GetQuote(),
			Signature: q.GetSignature(),
		}
	}
	return out
}

func pcrsToProto(in []attest.PCR) []*devicepb.TPMPCR {
	out := make([]*devicepb.TPMPCR, len(in))
	for i, pcr := range in {
		out[i] = devicepb.TPMPCR_builder{
			Index:     int32(pcr.Index),
			Digest:    pcr.Digest,
			DigestAlg: uint64(pcr.DigestAlg),
		}.Build()
	}
	return out
}

func pcrsFromProto(in []*devicepb.TPMPCR) []attest.PCR {
	out := make([]attest.PCR, len(in))
	for i, pcr := range in {
		out[i] = attest.PCR{
			Index:     int(pcr.GetIndex()),
			Digest:    pcr.GetDigest(),
			DigestAlg: crypto.Hash(pcr.GetDigestAlg()),
		}
	}
	return out
}
