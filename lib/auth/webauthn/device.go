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
		var pubKey []byte
		if !idOnly {
			var err error
			pubKey, err = keyDERToCBOR(dev.U2F.PubKey)
			if err != nil {
				log.Warnf("WebAuthn: failed to convert U2F device key to CBOR: %v", err)
				return wan.Credential{}, false
			}
		}

		var authenticator wan.Authenticator
		if !idOnly {
			authenticator = wan.Authenticator{
				SignCount: dev.U2F.Counter,
			}
		}

		return wan.Credential{
			ID:            dev.U2F.KeyHandle,
			PublicKey:     pubKey,
			Authenticator: authenticator,
		}, true
	default:
		return wan.Credential{}, false
	}
}

func keyDERToCBOR(der []byte) ([]byte, error) {
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
