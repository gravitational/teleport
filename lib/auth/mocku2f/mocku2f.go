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

package mocku2f

/* Mock U2F device for testing.
 * This is not a complete implementation of U2F keys.
 * In particular, the key only supports a single key handle that is specified upon creation
 */

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"math/big"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/cryptopatch"
)

// u2fRegistrationFlags is fixed by the U2F standard.
// https://fidoalliance.org/specs/fido-u2f-v1.2-ps-20170411/fido-u2f-raw-message-formats-v1.2-ps-20170411.html#registration-response-message-success
const u2fRegistrationFlags = 0x05

type Key struct {
	KeyHandle  []byte
	PrivateKey *ecdsa.PrivateKey

	// Cert is the Key attestation certificate.
	Cert []byte

	// UserHandle is the WebAuthn User ID.
	// Saved from passwordless registrations and set on passwordless assertions.
	// Requires a passwordless-configured Key (see [Key.SetPasswordless]).
	UserHandle []byte

	// PreferRPID instructs the Key to use favor using the RPID for Webauthn
	// ceremonies, even if the U2F App ID extension is present.
	PreferRPID bool
	// IgnoreAllowedCredentials allows the Key to sign a Webauthn
	// CredentialAssertion even it its KeyHandle is not among the allowed
	// credentials.
	IgnoreAllowedCredentials bool
	// SetUV sets the UV (user verification) bit on signatures if true.
	// SetUV should be paired only with WebAuthn login/registration methods, as
	// it makes Key mimic a WebAuthn device.
	SetUV bool
	// SetBackupFlags sets BE=1 and BS=1 in assertion responses.
	// - https://w3c.github.io/webauthn/#authdata-flags-be
	// - https://w3c.github.io/webauthn/#authdata-flags-bs
	SetBackupFlags bool
	// AllowResidentKey allows creation of resident credentials.
	// There's no actual change in Key's behavior other than allowing such requests
	// to proceed.
	// AllowResidentKey should be paired only with WebAuthn registration methods,
	// as it makes Key mimic a WebAuthn device.
	AllowResidentKey bool
	// ReplyWithCredProps sets the credProps extension (rk=true) in
	// SignCredentialCreation responses, regardless of other parameters.
	// Useful for extension testing.
	ReplyWithCredProps bool

	counter uint32
}

func selfSignPublicKey(keyToSign *ecdsa.PublicKey) (cert []byte, err error) {
	caPrivateKey, err := cryptopatch.GenerateECDSAKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return []byte{}, err
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test CA"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	cert, err = x509.CreateCertificate(rand.Reader, &template, &template, keyToSign, caPrivateKey)
	if err != nil {
		return []byte{}, err
	}
	return cert, nil
}

func Create() (*Key, error) {
	keyHandle := make([]byte, 128)
	_, err := rand.Read(keyHandle)
	if err != nil {
		return nil, err
	}
	return CreateWithKeyHandle(keyHandle)
}

func CreateWithKeyHandle(keyHandle []byte) (*Key, error) {
	if len(keyHandle) > 255 {
		return nil, trace.BadParameter("keyHandle length exceeds limit")
	}

	privatekey, err := cryptopatch.GenerateECDSAKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	cert, err := selfSignPublicKey(&privatekey.PublicKey)
	if err != nil {
		return nil, err
	}

	return &Key{
		KeyHandle:  keyHandle,
		PrivateKey: privatekey,
		Cert:       cert,
		counter:    1,
	}, nil
}

// SetPasswordless sets common passwordless options in Key.
// Options are AllowResidentKey, IgnoreAllowedCredentials and SetUV.
func (muk *Key) SetPasswordless() {
	muk.AllowResidentKey = true         // Passwordless keys must be resident.
	muk.IgnoreAllowedCredentials = true // Empty for passwordless challenges.
	muk.SetUV = true                    // UV required for passwordless.
}

// RegisterRaw signs low-level U2F registration data.
// Most callers should use either RegisterResponse or SignCredentialCreation.
func (muk *Key) RegisterRaw(appHash, challengeHash []byte) ([]byte, error) {
	res, err := muk.signRegister(appHash, challengeHash)
	if err != nil {
		return nil, err
	}
	return res.RawResp, nil
}

type signRegisterResult struct {
	RawResp   []byte
	Signature []byte
}

func (muk *Key) signRegister(appIDHash, clientDataHash []byte) (*signRegisterResult, error) {
	// Marshal pubKey into the uncompressed "4||X||Y" form.
	ecdhPubKey, err := muk.PrivateKey.PublicKey.ECDH()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pubKey := ecdhPubKey.Bytes()

	cap := 1 + len(appIDHash) + len(clientDataHash) + len(muk.KeyHandle) + len(pubKey)
	dataToSign := make([]byte, 0, cap)
	dataToSign = append(dataToSign, 0)
	dataToSign = append(dataToSign, appIDHash...)
	dataToSign = append(dataToSign, clientDataHash...)
	dataToSign = append(dataToSign, muk.KeyHandle...)
	dataToSign = append(dataToSign, pubKey...)

	dataHash := sha256.Sum256(dataToSign)

	// Despite taking a hash function, this actually does not hash the input.
	sig, err := muk.PrivateKey.Sign(rand.Reader, dataHash[:], crypto.SHA256)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	flags := uint8(u2fRegistrationFlags)
	if muk.SetUV {
		// Mimic WebAuthn flags if SetUV is true.
		flags = uint8(protocol.FlagUserPresent | protocol.FlagUserVerified | protocol.FlagAttestedCredentialData)
	}
	if muk.SetBackupFlags {
		flags |= uint8(protocol.FlagBackupEligible)
		flags |= uint8(protocol.FlagBackupState)
	}

	cap = 1 + len(pubKey) + 1 + len(muk.KeyHandle) + len(muk.Cert) + len(sig)
	regData := make([]byte, 0, cap)
	regData = append(regData, flags)
	regData = append(regData, pubKey...)
	regData = append(regData, byte(len(muk.KeyHandle)))
	regData = append(regData, muk.KeyHandle...)
	regData = append(regData, muk.Cert...)
	regData = append(regData, sig...)

	return &signRegisterResult{
		RawResp:   regData,
		Signature: sig,
	}, nil
}

// AuthenticateRaw signs low-level U2F authentication data.
// Most callers should use either SignResponse or SignAssertion.
func (muk *Key) AuthenticateRaw(appHash, challengeHash []byte) ([]byte, error) {
	res, err := muk.signAuthn(appHash, challengeHash)
	if err != nil {
		return nil, err
	}
	return res.SignData, nil
}

type signAuthnResult struct {
	// SignData is the signed data, which is a signature over the concatenation of
	// AuthData and ClientDataHash.
	// Includes the user presence bit, counter value and signature.
	// https://fidoalliance.org/specs/fido-u2f-v1.2-ps-20170411/fido-u2f-raw-message-formats-v1.2-ps-20170411.html#h3_authentication-response-message-success.
	SignData []byte
	// AuthData is the authenticator data generated by signAuthn and included in
	// the signature.
	AuthData []byte
}

// signAuthn signs appIDHash and clientDataHash for authentication purposes.
func (muk *Key) signAuthn(appIDHash, clientDataHash []byte) (*signAuthnResult, error) {
	counterBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(counterBytes, muk.counter)
	muk.counter++

	flags := uint8(protocol.FlagUserPresent)
	if muk.SetUV {
		flags |= uint8(protocol.FlagUserVerified)
	}
	if muk.SetBackupFlags {
		flags |= uint8(protocol.FlagBackupEligible)
		flags |= uint8(protocol.FlagBackupState)
	}

	var authData []byte
	authData = append(authData, appIDHash[:]...)
	authData = append(authData, flags)
	authData = append(authData, counterBytes[:]...)

	var dataToSign []byte
	dataToSign = append(dataToSign, authData[:]...)
	dataToSign = append(dataToSign, clientDataHash[:]...)

	dataHash := sha256.Sum256(dataToSign)

	// Despite taking a hash function, this actually does not hash the input.
	sig, err := muk.PrivateKey.Sign(rand.Reader, dataHash[:], crypto.SHA256)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var signData []byte
	signData = append(signData, flags)
	signData = append(signData, counterBytes[:]...)
	signData = append(signData, sig[:]...)

	return &signAuthnResult{
		SignData: signData,
		AuthData: authData,
	}, nil
}

func (muk *Key) Counter() uint32 {
	return muk.counter
}

func (muk *Key) SetCounter(counter uint32) {
	muk.counter = counter
}
