/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webauthn

import (
	"crypto/ecdsa"
	"crypto/x509"

	"github.com/duo-labs/webauthn/protocol/webauthncose"
	"github.com/fxamacker/cbor/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"

	wan "github.com/duo-labs/webauthn/webauthn"
	log "github.com/sirupsen/logrus"
)

// curveP256CBOR is the constant for the P-256 curve in CBOR.
// https://datatracker.ietf.org/doc/html/rfc8152#section-13.1
const curveP256CBOR = 1

func deviceToCredential(dev *types.MFADevice, idOnly bool) (wan.Credential, bool) {
	switch dev := dev.Device.(type) {
	case *types.MFADevice_U2F:
		var pubKeyCBOR []byte
		if !idOnly {
			var err error
			pubKeyCBOR, err = u2fDERKeyToCBOR(dev.U2F.PubKey)
			if err != nil {
				log.Warnf("WebAuthn: failed to convert U2F device key to CBOR: %v", err)
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
		return wan.Credential{
			ID:              dev.Webauthn.CredentialId,
			PublicKey:       pubKeyCBOR,
			AttestationType: dev.Webauthn.AttestationType,
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
		return nil, trace.Wrap(err)
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
