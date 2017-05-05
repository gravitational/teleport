package types

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"hash"
)

//EncryptedKey contains the decryption key data from the saml2 core and xmlenc
//standards.
type EncryptedKey struct {
	// EncryptionMethod string `xml:"EncryptionMethod>Algorithm"`
	X509Data         string `xml:"KeyInfo>X509Data>X509Certificate"`
	CipherValue      string `xml:"CipherData>CipherValue"`
	EncryptionMethod EncryptionMethod
}

//EncryptionMethod specifies the type of encryption that was used.
type EncryptionMethod struct {
	Algorithm    string `xml:",attr"`
	DigestMethod DigestMethod
}

//DigestMethod is a digest type specification
type DigestMethod struct {
	Algorithm string `xml:",attr"`
}

//Well-known public-key encryption methods
const (
	MethodRSAOAEP  = "http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p"
	MethodRSAOAEP2 = "http://www.w3.org/2009/xmlenc11#rsa-oaep"
)

//Well-known private key encryption methods
const (
	MethodAES128GCM = "http://www.w3.org/2009/xmlenc11#aes128-gcm"
	MethodAES128CBC = "http://www.w3.org/2001/04/xmlenc#aes128-cbc"
)

//Well-known hash methods
const (
	MethodSHA1   = "http://www.w3.org/2000/09/xmldsig#sha1"
	MethodSHA256 = "http://www.w3.org/2000/09/xmldsig#sha256"
	MethodSHA512 = "http://www.w3.org/2000/09/xmldsig#sha512"
)

//DecryptSymmetricKey returns the private key contained in the EncryptedKey document
func (ek *EncryptedKey) DecryptSymmetricKey(cert *tls.Certificate) (cipher.Block, error) {
	encCert, err := base64.StdEncoding.DecodeString(ek.X509Data)
	if err != nil {
		return nil, fmt.Errorf("error getting certificate from encryptedkey: %v", err)
	}

	if len(cert.Certificate) < 1 {
		return nil, fmt.Errorf("decryption tls.Certificate has no public certs attached")
	}

	if !bytes.Equal(cert.Certificate[0], encCert) {
		return nil, fmt.Errorf("key decryption attempted with mismatched cert: %#v != %#v",
			string(cert.Certificate[0]), string(encCert))
	}

	cipherText, err := base64.StdEncoding.DecodeString(ek.CipherValue)
	if err != nil {
		return nil, err
	}

	switch pk := cert.PrivateKey.(type) {
	case *rsa.PrivateKey:
		var h hash.Hash

		switch ek.EncryptionMethod.DigestMethod.Algorithm {
		case MethodSHA1:
			h = sha1.New()
		case MethodSHA256:
			h = sha256.New()
		case MethodSHA512:
			h = sha512.New()
		}

		switch ek.EncryptionMethod.Algorithm {
		case MethodRSAOAEP, MethodRSAOAEP2:
			pt, err := rsa.DecryptOAEP(h, rand.Reader, pk, cipherText, nil)
			if err != nil {
				return nil, fmt.Errorf("rsa internal error: %v", err)
			}

			b, err := aes.NewCipher(pt)
			if err != nil {
				return nil, err
			}

			return b, nil
		default:
			return nil, fmt.Errorf("unsupported encryption algorithm: %s", ek.EncryptionMethod.Algorithm)
		}
	}
	return nil, fmt.Errorf("no cipher for decoding symmetric key")
}
