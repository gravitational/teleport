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
