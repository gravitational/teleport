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

package webauthn

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"

	"github.com/fxamacker/cbor/v2"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	wan "github.com/go-webauthn/webauthn/webauthn"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// curveP256CBOR is the constant for the P-256 curve in CBOR.
// https://datatracker.ietf.org/doc/html/rfc8152#section-13.1
const curveP256CBOR = 1

type credentialFlags struct {
	BE, BS bool
}

func deviceToCredential(
	dev *types.MFADevice,
	idOnly bool,
	currentFlags *credentialFlags,
) (wan.Credential, bool) {
	switch dev := dev.Device.(type) {
	case *types.MFADevice_U2F:
		var pubKeyCBOR []byte
		if !idOnly {
			var err error
			pubKeyCBOR, err = u2fDERKeyToCBOR(dev.U2F.PubKey)
			if err != nil {
				log.WarnContext(context.Background(), "failed to convert U2F device key to CBOR", "error", err)
				return wan.Credential{}, false
			}
		}
		return wan.Credential{
			ID:        dev.U2F.KeyHandle,
			PublicKey: pubKeyCBOR,
			Authenticator: wan.Authenticator{
				SignCount: dev.U2F.Counter,
			},
		}, true
	case *types.MFADevice_Webauthn:
		var pubKeyCBOR []byte
		if !idOnly {
			pubKeyCBOR = dev.Webauthn.PublicKeyCbor
		}

		// Use BE/BS from the device, falling back to currentFlags for devices that
		// haven't been backfilled yet.
		var be, bs bool
		if dev.Webauthn.CredentialBackupEligible != nil {
			be = dev.Webauthn.CredentialBackupEligible.Value
		} else {
			be = currentFlags != nil && currentFlags.BE
		}
		if dev.Webauthn.CredentialBackedUp != nil {
			bs = dev.Webauthn.CredentialBackedUp.Value
		} else {
			bs = currentFlags != nil && currentFlags.BS
		}

		return wan.Credential{
			ID:              dev.Webauthn.CredentialId,
			PublicKey:       pubKeyCBOR,
			AttestationType: dev.Webauthn.AttestationType,
			Flags: wan.CredentialFlags{
				BackupEligible: be,
				BackupState:    bs,
			},
			Authenticator: wan.Authenticator{
				AAGUID:    dev.Webauthn.Aaguid,
				SignCount: dev.Webauthn.SignatureCounter,
			},
		}, true
	default:
		return wan.Credential{}, false
	}
}

func u2fDERKeyToCBOR(der []byte) ([]byte, error) {
	pubKeyI, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// U2F device keys are guaranteed to be ECDSA/P256
	// https://fidoalliance.org/specs/fido-u2f-v1.2-ps-20170411/fido-u2f-raw-message-formats-v1.2-ps-20170411.html#h3_registration-response-message-success.
	pubKey, ok := pubKeyI.(*ecdsa.PublicKey)
	if !ok {
		return nil, trace.BadParameter("U2F public key has an unexpected type: %T", pubKeyI)
	}
	return U2FKeyToCBOR(pubKey)
}

// U2FKeyToCBOR transforms a DER-encoded U2F into its CBOR counterpart.
func U2FKeyToCBOR(pubKey *ecdsa.PublicKey) ([]byte, error) {
	// X and Y coordinates must be exactly 32 bytes.
	xBytes := make([]byte, 32)
	yBytes := make([]byte, 32)
	pubKey.X.FillBytes(xBytes)
	pubKey.Y.FillBytes(yBytes)

	pubKeyCBOR, err := cbor.Marshal(&webauthncose.EC2PublicKeyData{
		PublicKeyData: webauthncose.PublicKeyData{
			KeyType:   int64(webauthncose.EllipticKey),
			Algorithm: int64(webauthncose.AlgES256),
		},
		Curve:  curveP256CBOR,
		XCoord: xBytes,
		YCoord: yBytes,
	})
	return pubKeyCBOR, trace.Wrap(err)
}
