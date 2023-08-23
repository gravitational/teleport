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

package devicetrust_test

import (
	"crypto"
	"testing"

	"github.com/google/go-attestation/attest"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/devicetrust"
)

func TestAttestationParametersProto(t *testing.T) {
	want := attest.AttestationParameters{
		Public:            []byte("public"),
		CreateData:        []byte("create_data"),
		CreateAttestation: []byte("create_attestation"),
		CreateSignature:   []byte("create_signature"),
	}
	pb := devicetrust.AttestationParametersToProto(want)
	clonedPb := utils.CloneProtoMsg(pb)
	got := devicetrust.AttestationParametersFromProto(clonedPb)
	require.Equal(t, want, got)
}

func TestEncryptedCredentialProto(t *testing.T) {
	want := &attest.EncryptedCredential{
		Credential: []byte("encrypted_credential"),
		Secret:     []byte("secret"),
	}
	pb := devicetrust.EncryptedCredentialToProto(want)
	clonedPb := utils.CloneProtoMsg(pb)
	got := devicetrust.EncryptedCredentialFromProto(clonedPb)
	require.Equal(t, want, got)
}

func TestPlatformParametersProto(t *testing.T) {
	want := &attest.PlatformParameters{
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
				Digest:    []byte("digest_sha256_0"),
				DigestAlg: crypto.SHA256,
			},
			{
				Index:     0,
				Digest:    []byte("digest_sha1_0"),
				DigestAlg: crypto.SHA1,
			},
		},
	}
	pb := devicetrust.PlatformParametersToProto(want)
	clonedPb := utils.CloneProtoMsg(pb)
	got := devicetrust.PlatformParametersFromProto(clonedPb)
	// We expect `Public` to be nil because we don't transmit this field over
	// the wire. This is because we don't use this value and rely on our stored
	// version of the key.
	want.Public = nil
	require.Equal(t, want, got)
}

func TestPlatformAttestationProto(t *testing.T) {
	want := &attest.PlatformParameters{
		TPMVersion: attest.TPMVersion20,
		EventLog:   []byte("event_log"),
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
				Digest:    []byte("digest_sha256_0"),
				DigestAlg: crypto.SHA256,
			},
			{
				Index:     1,
				Digest:    []byte("digest_sha1_0"),
				DigestAlg: crypto.SHA1,
			},
		},
	}
	wantNonce := []byte("foo-bar-bizz-boo")
	pb := devicetrust.PlatformAttestationToProto(want, wantNonce)
	clonedPb := utils.CloneProtoMsg(pb)
	got, gotNonce := devicetrust.PlatformAttestationFromProto(clonedPb)
	require.Equal(t, want, got)
	require.Equal(t, wantNonce, gotNonce)
}
