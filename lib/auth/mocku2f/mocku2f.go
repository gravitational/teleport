/*
Copyright 2015 Gravitational, Inc.

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

package mocku2f

/* Mock U2F device for testing.
 * This is not a complete implementation of U2F keys.
 * In particular, the key only supports a single key handle that is specified upon creation
 */

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math/big"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/tstranex/u2f"
)

type Key struct {
	KeyHandle  []byte
	PrivateKey *ecdsa.PrivateKey

	// Cert is the Key attestation certificate.
	Cert []byte

	// PreferRPID instructs the Key to use favor using the RPID for Webauthn
	// ceremonies, even if the U2F App ID extension is present.
	PreferRPID bool

	// IgnoreAllowedCredentials allows the Key to sign a Webauthn
	// CredentialAssertion even it its KeyHandle is not among the allowed
	// credentials.
	IgnoreAllowedCredentials bool

	counter uint32
}

// The "websafe-base64 encoding" in the U2F specifications removes the padding
func decodeBase64(s string) ([]byte, error) {
	for i := 0; i < len(s)%4; i++ {
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}

func encodeBase64(buf []byte) string {
	s := base64.URLEncoding.EncodeToString(buf)
	return strings.TrimRight(s, "=")
}

func selfSignPublicKey(keyToSign *ecdsa.PublicKey) (cert []byte, err error) {
	caPrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
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

	privatekey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
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

func (muk *Key) RegisterResponse(req *u2f.RegisterRequest) (*u2f.RegisterResponse, error) {
	appIDHash := sha256.Sum256([]byte(req.AppID))

	clientData := u2f.ClientData{
		Typ:       "navigator.id.finishEnrollment",
		Challenge: req.Challenge,
		Origin:    req.AppID,
	}
	clientDataJSON, err := json.Marshal(clientData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clientDataHash := sha256.Sum256(clientDataJSON)

	res, err := muk.signRegister(appIDHash[:], clientDataHash[:])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &u2f.RegisterResponse{
		RegistrationData: encodeBase64(res.RawResp),
		ClientData:       encodeBase64(clientDataJSON),
	}, nil
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
	pubKey := elliptic.Marshal(elliptic.P256(), muk.PrivateKey.PublicKey.X, muk.PrivateKey.PublicKey.Y)

	var dataToSign []byte
	dataToSign = append(dataToSign[:], 0)
	dataToSign = append(dataToSign[:], appIDHash[:]...)
	dataToSign = append(dataToSign[:], clientDataHash[:]...)
	dataToSign = append(dataToSign[:], muk.KeyHandle[:]...)
	dataToSign = append(dataToSign[:], pubKey[:]...)

	dataHash := sha256.Sum256(dataToSign)

	// Despite taking a hash function, this actually does not hash the input.
	sig, err := muk.PrivateKey.Sign(rand.Reader, dataHash[:], crypto.SHA256)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var regData []byte
	regData = append(regData, 5) // fixed by specification
	regData = append(regData, pubKey[:]...)
	regData = append(regData, byte(len(muk.KeyHandle)))
	regData = append(regData, muk.KeyHandle[:]...)
	regData = append(regData, muk.Cert[:]...)
	regData = append(regData, sig[:]...)

	return &signRegisterResult{
		RawResp:   regData,
		Signature: sig,
	}, nil
}

func (muk *Key) SignResponse(req *u2f.SignRequest) (*u2f.SignResponse, error) {
	rawKeyHandle, err := decodeBase64(req.KeyHandle)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !bytes.Equal(rawKeyHandle, muk.KeyHandle) {
		return nil, trace.CompareFailed("wrong keyHandle")
	}
	appIDHash := sha256.Sum256([]byte(req.AppID))

	clientData := u2f.ClientData{
		Typ:       "navigator.id.getAssertion",
		Challenge: req.Challenge,
		Origin:    req.AppID,
	}
	clientDataJSON, err := json.Marshal(clientData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clientDataHash := sha256.Sum256(clientDataJSON)

	res, err := muk.signAuthn(appIDHash[:], clientDataHash[:])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &u2f.SignResponse{
		KeyHandle:     req.KeyHandle,
		SignatureData: encodeBase64(res.SignData),
		ClientData:    encodeBase64(clientDataJSON),
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

	var authData []byte
	authData = append(authData, appIDHash[:]...)
	authData = append(authData, 1) // user presence
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
	signData = append(signData, 1) // user presence
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
