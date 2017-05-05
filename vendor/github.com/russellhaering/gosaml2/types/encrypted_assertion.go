package types

import (
	"bytes"
	"crypto/cipher"
	"crypto/tls"
	"encoding/base64"
	"encoding/xml"
	"fmt"
)

type EncryptedAssertion struct {
	XMLName          xml.Name         `xml:"urn:oasis:names:tc:SAML:2.0:assertion EncryptedAssertion"`
	EncryptionMethod EncryptionMethod `xml:"EncryptedData>EncryptionMethod"`
	EncryptedKey     EncryptedKey     `xml:"EncryptedData>KeyInfo>EncryptedKey"`
	CipherValue      string           `xml:"EncryptedData>CipherData>CipherValue"`
}

func (ea *EncryptedAssertion) decrypt(cert *tls.Certificate) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(ea.CipherValue)
	if err != nil {
		return nil, err
	}

	k, err := ea.EncryptedKey.DecryptSymmetricKey(cert)
	if err != nil {
		return nil, fmt.Errorf("cannot decrypt, error retrieving private key: %s", err)
	}

	switch ea.EncryptionMethod.Algorithm {
	case MethodAES128GCM:
		c, err := cipher.NewGCM(k)
		if err != nil {
			return nil, fmt.Errorf("cannot create AES-GCM: %s", err)
		}

		nonce, data := data[:c.NonceSize()], data[c.NonceSize():]
		plainText, err := c.Open(nil, nonce, data, nil)
		if err != nil {
			return nil, fmt.Errorf("cannot open AES-GCM: %s", err)
		}
		return plainText, nil
	case MethodAES128CBC:
		nonce, data := data[:k.BlockSize()], data[k.BlockSize():]
		c := cipher.NewCBCDecrypter(k, nonce)
		c.CryptBlocks(data, data)

		// Remove zero bytes
		data = bytes.TrimRight(data, "\x00")

		// Calculate index to remove based on padding
		padLength := data[len(data)-1]
		lastGoodIndex := len(data) - int(padLength)
		return data[:lastGoodIndex], nil
	default:
		return nil, fmt.Errorf("unknown symmetric encryption method %#v", ea.EncryptionMethod.Algorithm)
	}
}

// Decrypt decrypts and unmarshals the EncryptedAssertion.
func (ea *EncryptedAssertion) Decrypt(cert *tls.Certificate) (*Assertion, error) {
	plaintext, err := ea.decrypt(cert)

	assertion := &Assertion{}

	err = xml.Unmarshal(plaintext, assertion)
	if err != nil {
		return nil, fmt.Errorf("Error decrypting assertion: %v", err)
	}

	return assertion, nil
}
