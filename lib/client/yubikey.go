/*
Copyright 2022 Gravitational, Inc.

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

package client

import (
	"crypto"
	"crypto/rand"
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-piv/piv-go/piv"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	YubikeyPolicyNever  = "never"
	YubikeyPolicyOnce   = "once"
	YubikeyPolicyAlways = "always"
)

var YubikeyPolicyOptions = []string{YubikeyPolicyNever, YubikeyPolicyOnce, YubikeyPolicyAlways}

func ParseYubikeyPinPolicy(policy string) (piv.PINPolicy, error) {
	switch policy {
	case YubikeyPolicyNever:
		return piv.PINPolicyNever, nil
	case YubikeyPolicyOnce:
		return piv.PINPolicyOnce, nil
	case YubikeyPolicyAlways:
		return piv.PINPolicyAlways, nil
	default:
		return 0, trace.BadParameter("invalid yubikey pin policy  %q, must be one of %v", policy, YubikeyPolicyOptions)
	}
}

func ParseYubikeyTouchPolicy(policy string) (piv.TouchPolicy, error) {
	switch policy {
	case YubikeyPolicyNever:
		return piv.TouchPolicyNever, nil
	case YubikeyPolicyOnce:
		return piv.TouchPolicyCached, nil
	case YubikeyPolicyAlways:
		return piv.TouchPolicyAlways, nil
	default:
		return 0, trace.BadParameter("invalid yubikey touch policy %q, must be one of %v", policy, YubikeyPolicyOptions)
	}
}

// YkPrivateKey is a keypair generated and held on a yubikey
type YkPrivateKey struct {
	card        string
	pub         crypto.PublicKey
	pinPolicy   piv.PINPolicy
	touchPolicy piv.TouchPolicy
}

// GetYkPrivateKey connects to a yubikey to get private key information
// that can be used for subsequent key operations.
//
// First, we grab the certificate on the given yubikey slot, followed
// by its attestation cert to verify the cert was generated with a real
// yubikey key (not imported). If verified, we take the public key from
// the cert for further key operations.
func GetYkPrivateKey() (*YkPrivateKey, error) {
	card, err := GetYubikeyCard()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	yk, err := openYubikey(card)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer yk.Close()

	attestationCert, err := yk.AttestationCertificate()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	slotCert, err := yk.Attest(piv.SlotAuthentication)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	attestation, err := piv.Verify(attestationCert, slotCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &YkPrivateKey{
		card:        card,
		pub:         slotCert.PublicKey,
		pinPolicy:   attestation.PINPolicy,
		touchPolicy: attestation.TouchPolicy,
	}, nil
}

func GenerateYkPrivateKey(pinPolicy piv.PINPolicy, touchPolicy piv.TouchPolicy) (*YkPrivateKey, error) {
	card, err := GetYubikeyCard()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	yk, err := openYubikey(card)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer yk.Close()

	// TODO: slot key is already set, do we want to overwrite? prompt user?
	// if _, err := yk.Attest(piv.SlotAuthentication); err == nil {
	// 	// slot in use
	// }

	// TODO: prompt user to set pin and management key
	// yk.SetPIN()
	// yk.SetManagementKey()

	// TODO: use management key from user input or randomly generated
	managementKey := piv.DefaultManagementKey

	fmt.Println("generating yubikey private key")
	pub, err := yk.GenerateKey(managementKey, piv.SlotAuthentication, piv.Key{
		Algorithm:   piv.AlgorithmEC256,
		PINPolicy:   pinPolicy,
		TouchPolicy: touchPolicy,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Cache the user's touch now by performing a fake signature.
	if touchPolicy == piv.TouchPolicyCached {
		// TODO: use pin set above
		auth := piv.KeyAuth{PIN: piv.DefaultPIN}

		priv, err := yk.PrivateKey(piv.SlotAuthentication, pub, auth)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		fmt.Printf("\ntap your yubikey\n")
		priv.(crypto.Signer).Sign(rand.Reader, []byte{}, nil)
	}

	return &YkPrivateKey{
		card:        card,
		pub:         pub,
		pinPolicy:   pinPolicy,
		touchPolicy: touchPolicy,
	}, nil
}

func (pk *YkPrivateKey) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	yk, err := openYubikey(pk.card)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer yk.Close()

	// TODO: prompt user for pin
	auth := piv.KeyAuth{PIN: piv.DefaultPIN}

	privateKey, err := yk.PrivateKey(piv.SlotAuthentication, pk.pub, auth)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if pk.touchPolicy == piv.TouchPolicyAlways {
		fmt.Printf("\ntap your yubikey\n")
	}

	return privateKey.(crypto.Signer).Sign(rand, digest, opts)
}

const yubikeyKeyDataPrefix = "yubikey-key"

func (pk *YkPrivateKey) PrivateKeyData() []byte {
	return []byte(fmt.Sprintf("%s %s", yubikeyKeyDataPrefix, pk.card))
}

func (pk *YkPrivateKey) PrivateKeyPEMTODO() []byte {
	return pk.PrivateKeyData()
}

func (pk *YkPrivateKey) Equal(x crypto.PrivateKey) bool {
	// TODO: piv-go doesn't implement Equal
	return true
}

func (pk *YkPrivateKey) Public() crypto.PublicKey {
	return pk.pub
}

func (pk *YkPrivateKey) TLSCertificate(cert []byte) (tls.Certificate, error) {
	certPEMBlock, _ := pem.Decode(cert)
	return tls.Certificate{
		Certificate: [][]byte{certPEMBlock.Bytes},
		PrivateKey:  pk,
	}, nil
}

func (pk *YkPrivateKey) AttestationCerts() ([]byte, []byte, error) {
	yk, err := openYubikey(pk.card)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer yk.Close()

	cert, err := yk.Attest(piv.SlotAuthentication)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	attestationCert, err := yk.AttestationCertificate()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return cert.Raw, attestationCert.Raw, nil
}

func (pk *YkPrivateKey) AsAgentKeys(cert *ssh.Certificate) []agent.AddedKey {
	return []agent.AddedKey{}
}

// make sure to close returned yubikey
func openYubikey(card string) (yk *piv.YubiKey, err error) {
	// TODO use proper retry logic
	isRetryError := func(err error) bool {
		retryError := "connecting to smart card: the smart card cannot be accessed because of other connections outstanding"
		return strings.Contains(err.Error(), retryError)
	}

	var maxRetries int = 100
	for i := 0; i < maxRetries; i++ {
		yk, err = piv.Open(card)
		if err == nil {
			return yk, nil
		}

		if !isRetryError(err) {
			return nil, trace.Wrap(err)
		}

		time.Sleep(time.Millisecond * 100)
	}

	return nil, trace.Wrap(err)
}

func GetYubikeyCard() (string, error) {
	cards, err := piv.Cards()
	if err != nil {
		return "", trace.Wrap(err)
	}

	var yubikeyCards []string
	for _, card := range cards {
		if strings.Contains(strings.ToLower(card), "yubikey") {
			yubikeyCards = append(yubikeyCards, card)
		}
	}

	switch len(yubikeyCards) {
	case 0:
		return "", trace.NotFound("no yubikey device detected")
	case 1:
		return yubikeyCards[0], nil
	default:
		return "", trace.BadParameter("more than one yubikey device detected, please connect one yubikey you wish to use")
	}
}
