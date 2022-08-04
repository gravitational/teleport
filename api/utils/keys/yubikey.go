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

package keys

import (
	"bytes"
	"context"
	"crypto"
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/utils/prompt"

	"github.com/go-piv/piv-go/piv"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func GenerateYubikeyPrivateKey(serialNumber string, slot piv.Slot, pinPolicy piv.PINPolicy, touchPolicy piv.TouchPolicy) (*YubikeyPrivateKey, error) {
	y, err := getYubikeySlot(serialNumber, slot)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	yk, err := y.openPIV()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer yk.Close()

	// TODO: slot key is already set, do we want to overwrite? prompt user?
	// if _, err := yk.Attest(slot); err == nil {
	// 	// slot in use
	// }

	// TODO: get mgm key from PIN protected metadata, or prompt user
	managementKey := piv.DefaultManagementKey

	fmt.Println("generating yubikey private key")
	pub, err := yk.GenerateKey(managementKey, y.slot, piv.Key{
		Algorithm:   piv.AlgorithmEC256,
		PINPolicy:   pinPolicy,
		TouchPolicy: touchPolicy,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &YubikeyPrivateKey{
		yubikeySlot: y,
		pub:         pub,
	}, nil
}

func GetYubikeyPrivateKey(serialNumber string, slot piv.Slot) (*YubikeyPrivateKey, error) {
	y, err := getYubikeySlot(serialNumber, slot)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	yk, err := y.openPIV()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer yk.Close()

	slotCert, err := yk.Attest(y.slot)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &YubikeyPrivateKey{
		yubikeySlot: y,
		pivSlot:     y.slot,
		pub:         slotCert.PublicKey,
	}, nil
}

// Returns a new RSAPrivateKey from an existing PEM-encoded RSA key pair.
func ParseYubikeyPrivateKey(priv []byte) (*YubikeyPrivateKey, error) {
	data := strings.Split(string(priv), "+")
	if len(data) != 2 {
		return nil, trace.BadParameter("")
	}

	serialNumber := data[0]
	pivSlot, err := ParsePIVSlot(data[1])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return GetYubikeyPrivateKey(serialNumber, pivSlot)
}

type yubikeySlot struct {
	yubikey
	slot piv.Slot
}

func getYubikeySlot(serialNumber string, slot piv.Slot) (*yubikeySlot, error) {
	yubikeys, err := findYubikeys()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(yubikeys) == 0 {
		return nil, trace.NotFound("no yubikey devices found")
	}

	if serialNumber == "" {
		return &yubikeySlot{
			yubikey: yubikeys[0],
			slot:    slot,
		}, nil
	}

	for _, yk := range yubikeys {
		if yk.serialNumber == serialNumber {
			return &yubikeySlot{
				yubikey: yk,
				slot:    slot,
			}, nil
		}
	}

	return nil, trace.NotFound("no yubikey devices found with serial number %q", serialNumber)
}

func (y yubikeySlot) IsOpen() (bool, error) {
	// TODO: check if slot is in use by a tsh login session
	return false, nil
}

type yubikey struct {
	card         string
	serialNumber string
}

func (y yubikey) openPIV() (yk *piv.YubiKey, err error) {
	return openYubikeyPIV(y.card)
}

type YubikeyPrivateKey struct {
	*yubikeySlot
	pivSlot piv.Slot
	pub     crypto.PublicKey
}

func (pk *YubikeyPrivateKey) Public() crypto.PublicKey {
	return pk.pub
}

func (pk *YubikeyPrivateKey) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	yk, err := pk.openPIV()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer yk.Close()

	auth := piv.KeyAuth{PINPrompt: func() (pin string, err error) {
		return prompt.Password(context.Background(), os.Stderr, prompt.Stdin(), "Enter the PIV card's PIN")
	}}

	privateKey, err := yk.PrivateKey(pk.pivSlot, pk.pub, auth)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return privateKey.(crypto.Signer).Sign(rand, digest, opts)
}

func (pk *YubikeyPrivateKey) Equal(x crypto.PrivateKey) bool {
	switch other := x.(type) {
	case *YubikeyPrivateKey:
		return bytes.Equal(pk.PrivateKeyDataPEM(), other.PrivateKeyDataPEM())
	default:
		return false
	}
}

func (pk *YubikeyPrivateKey) PrivateKeyDataPEM() []byte {
	data := fmt.Sprintf("%s+%s", pk.serialNumber, pk.slot.String())
	return pem.EncodeToMemory(&pem.Block{
		Type:    pivYubikeyPrivateKeyType,
		Headers: nil,
		Bytes:   []byte(data),
	})
}

func (pk *YubikeyPrivateKey) TLSCertificate(cert []byte) (tls.Certificate, error) {
	certPEMBlock, _ := pem.Decode(cert)
	return tls.Certificate{
		Certificate: [][]byte{certPEMBlock.Bytes},
		PrivateKey:  pk,
	}, nil
}

func (pk *YubikeyPrivateKey) AsAgentKeys(cert *ssh.Certificate) []agent.AddedKey {
	return []agent.AddedKey{}
}

func findYubikeys() ([]yubikey, error) {
	cards, err := piv.Cards()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var yubikeys []yubikey
	for _, card := range cards {
		if strings.Contains(strings.ToLower(card), PIVCardTypeYubikey) {
			serialNumber, err := getSerialNumber(card)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			yubikeys = append(yubikeys, yubikey{
				card:         card,
				serialNumber: serialNumber,
			})
		}
	}

	return yubikeys, nil
}

func getSerialNumber(card string) (string, error) {
	yk, err := openYubikeyPIV(card)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer yk.Close()

	serialNumber, err := yk.Serial()
	if err != nil {
		return "", trace.Wrap(err)
	}

	return strconv.FormatUint(uint64(serialNumber), 10), nil
}

func openYubikeyPIV(card string) (yk *piv.YubiKey, err error) {
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
