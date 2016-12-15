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

	"github.com/tstranex/u2f"
	"github.com/gravitational/trace"
)

type Key struct {
	keyHandle []byte
	privatekey *ecdsa.PrivateKey
	cert []byte
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
		NotBefore: time.Now(),
		NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA: true,
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
		keyHandle: keyHandle,
		privatekey: privatekey,
		cert: cert,
		counter: 1,
	}, nil
}

func (muk *Key) RegisterResponse(req *u2f.RegisterRequest) (*u2f.RegisterResponse, error) {
	appIDHash := sha256.Sum256([]byte(req.AppID))

	clientData := u2f.ClientData{
		Typ: "navigator.id.finishEnrollment",
		Challenge: req.Challenge,
		Origin: req.AppID,
	}
	clientDataJson, err := json.Marshal(clientData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clientDataHash := sha256.Sum256(clientDataJson)

	marshalledPublickey := elliptic.Marshal(elliptic.P256(), muk.privatekey.PublicKey.X, muk.privatekey.PublicKey.Y)

	var dataToSign []byte
	dataToSign = append(dataToSign[:], []byte{ 0 }[:]...)
	dataToSign = append(dataToSign[:], appIDHash[:]...)
	dataToSign = append(dataToSign[:], clientDataHash[:]...)
	dataToSign = append(dataToSign[:], muk.keyHandle[:]...)
	dataToSign = append(dataToSign[:], marshalledPublickey[:]...)

	dataHash := sha256.Sum256(dataToSign)

	// Despite taking a hash function, this actually does not hash the input.
	sig, err := muk.privatekey.Sign(rand.Reader, dataHash[:], crypto.SHA256)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var regData []byte
	regData  = append(regData, []byte{ 5 }[:]...) // fixed by specification
	regData = append(regData, marshalledPublickey[:]...)
	regData = append(regData, []byte{ byte(len(muk.keyHandle)) }[:]...)
	regData = append(regData, muk.keyHandle[:]...)
	regData = append(regData, muk.cert[:]...)
	regData = append(regData, sig[:]...)

	return &u2f.RegisterResponse{
		RegistrationData: encodeBase64(regData),
		ClientData: encodeBase64(clientDataJson),
	}, nil
}

func (muk *Key) SignResponse(req *u2f.SignRequest) (*u2f.SignResponse, error) {
	rawKeyHandle, err := decodeBase64(req.KeyHandle)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !bytes.Equal(rawKeyHandle, muk.keyHandle) {
		return nil, trace.CompareFailed("wrong keyHandle")
	}

	appIDHash := sha256.Sum256([]byte(req.AppID))

	counterBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(counterBytes, muk.counter)
	muk.counter += 1

	clientData := u2f.ClientData{
		Typ: "navigator.id.getAssertion",
		Challenge: req.Challenge,
		Origin: req.AppID,
	}
	clientDataJson, err := json.Marshal(clientData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clientDataHash := sha256.Sum256(clientDataJson)

	var dataToSign []byte
	dataToSign = append(dataToSign, appIDHash[:]...)
	dataToSign = append(dataToSign, []byte{ 1 }[:]...) // user presence
	dataToSign = append(dataToSign, counterBytes[:]...)
	dataToSign = append(dataToSign, clientDataHash[:]...)

	dataHash := sha256.Sum256(dataToSign)

	// Despite taking a hash function, this actually does not hash the input.
	sig, err := muk.privatekey.Sign(rand.Reader, dataHash[:], crypto.SHA256)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var signData []byte
	signData = append(signData, []byte{ 1 }[:]...) // user presence
	signData = append(signData, counterBytes[:]...)
	signData = append(signData, sig[:]...)

	return &u2f.SignResponse{
		KeyHandle: req.KeyHandle,
		SignatureData: encodeBase64(signData),
		ClientData: encodeBase64(clientDataJson),
	}, nil
}

func (muk *Key) SetCounter(counter uint32) {
	muk.counter = counter
}

