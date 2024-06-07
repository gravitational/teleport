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
	"testing"

	"github.com/google/go-attestation/attest"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils"
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
	want := &attest.EncryptedCredential{
		Credential: []byte("encrypted_credential"),
		Secret:     []byte("secret"),
	}
	pb := EncryptedCredentialToProto(want)
	clonedPb := utils.CloneProtoMsg(pb)
	got := EncryptedCredentialFromProto(clonedPb)
	require.Equal(t, want, got)
}
