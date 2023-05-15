// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package devicetrust

import (
	"crypto"

	"github.com/google/go-attestation/attest"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

func AttestationParametersToProto(in attest.AttestationParameters) *devicepb.TPMAttestationParameters {
	return &devicepb.TPMAttestationParameters{
		Public:            in.Public,
		CreateData:        in.CreateData,
		CreateAttestation: in.CreateAttestation,
		CreateSignature:   in.CreateSignature,
	}
}

func AttestationParametersFromProto(in *devicepb.TPMAttestationParameters) attest.AttestationParameters {
	return attest.AttestationParameters{
		Public:            in.Public,
		CreateData:        in.CreateData,
		CreateAttestation: in.CreateAttestation,
		CreateSignature:   in.CreateSignature,
	}
}

func EncryptedCredentialToProto(in *attest.EncryptedCredential) *devicepb.TPMEncryptedCredential {
	return &devicepb.TPMEncryptedCredential{
		CredentialBlob: in.Credential,
		Secret:         in.Secret,
	}
}

func EncryptedCredentialFromProto(in *devicepb.TPMEncryptedCredential) attest.EncryptedCredential {
	return attest.EncryptedCredential{
		Credential: in.CredentialBlob,
		Secret:     in.Secret,
	}
}

func PlatformParametersToProto(in *attest.PlatformParameters) *devicepb.TPMPlatformParameters {
	return &devicepb.TPMPlatformParameters{
		EventLog: in.EventLog,
		Quotes:   quotesToProto(in.Quotes),
		Pcrs:     pcrsToProto(in.PCRs),
	}
}

func PlatformParametersFromProto(in *devicepb.TPMPlatformParameters) attest.PlatformParameters {
	return attest.PlatformParameters{
		TPMVersion: attest.TPMVersion20,
		Quotes:     quotesFromProto(in.Quotes),
		PCRs:       pcrsFromProto(in.Pcrs),
		EventLog:   in.EventLog,
	}
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
