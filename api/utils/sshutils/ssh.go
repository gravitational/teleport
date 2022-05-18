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

// Package sshutils defines several functions and types used across the
// Teleport API and other Teleport packages when working with SSH.
package sshutils

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"runtime"
	"strings"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// ParseCertificate parses an SSH certificate from the authorized_keys format.
func ParseCertificate(buf []byte) (*ssh.Certificate, error) {
	k, _, _, _, err := ssh.ParseAuthorizedKey(buf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, ok := k.(*ssh.Certificate)
	if !ok {
		return nil, trace.BadParameter("not an SSH certificate")
	}

	return cert, nil
}

// ParseKnownHosts parses provided known_hosts entries into ssh.PublicKey list.
func ParseKnownHosts(knownHosts [][]byte) ([]ssh.PublicKey, error) {
	var keys []ssh.PublicKey
	for _, line := range knownHosts {
		for {
			_, _, publicKey, _, bytes, err := ssh.ParseKnownHosts(line)
			if err == io.EOF {
				break
			} else if err != nil {
				return nil, trace.Wrap(err, "failed parsing known hosts: %v; raw line: %q", err, line)
			}
			keys = append(keys, publicKey)
			line = bytes
		}
	}
	return keys, nil
}

// ParseAuthorizedKeys parses provided authorized_keys entries into ssh.PublicKey list.
func ParseAuthorizedKeys(authorizedKeys [][]byte) ([]ssh.PublicKey, error) {
	var keys []ssh.PublicKey
	for _, line := range authorizedKeys {
		publicKey, _, _, _, err := ssh.ParseAuthorizedKey(line)
		if err != nil {
			return nil, trace.Wrap(err, "failed parsing authorized keys: %v; raw line: %q", err, line)
		}
		keys = append(keys, publicKey)
	}
	return keys, nil
}

// ProxyClientSSHConfig returns an ssh.ClientConfig with SSH credentials from this
// Key and HostKeyCallback matching SSH CAs in the Key.
//
// The config is set up to authenticate to proxy with the first available principal.
//
func ProxyClientSSHConfig(sshCert, privKey []byte, caCerts [][]byte) (*ssh.ClientConfig, error) {
	cert, err := ParseCertificate(sshCert)
	if err != nil {
		return nil, trace.Wrap(err, "failed to extract username from SSH certificate")
	}

	authMethod, err := AsAuthMethod(cert, privKey)
	if err != nil {
		return nil, trace.Wrap(err, "failed to convert key pair to auth method")
	}

	hostKeyCallback, err := HostKeyCallback(caCerts, false)
	if err != nil {
		return nil, trace.Wrap(err, "failed to convert certificate authorities to HostKeyCallback")
	}

	// The KeyId is not always a valid principal, so we use the first valid principal instead.
	user := cert.KeyId
	if len(cert.ValidPrincipals) > 0 {
		user = cert.ValidPrincipals[0]
	}

	return &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: hostKeyCallback,
		Timeout:         defaults.DefaultDialTimeout,
	}, nil
}

// AsSigner returns an ssh.Signer from raw marshaled key and certificate.
func AsSigner(sshCert *ssh.Certificate, privKey []byte) (ssh.Signer, error) {
	keys, err := AsAgentKeys(sshCert, privKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signer, err := ssh.NewSignerFromKey(keys[0].PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signer, err = ssh.NewCertSigner(keys[0].Certificate, signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return signer, nil
}

// AsAuthMethod returns an "auth method" interface, a common abstraction
// used by Golang SSH library. This is how you actually use a Key to feed
// it into the SSH lib.
func AsAuthMethod(sshCert *ssh.Certificate, privKey []byte) (ssh.AuthMethod, error) {
	signer, err := AsSigner(sshCert, privKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ssh.PublicKeys(signer), nil
}

// AsAgentKeys converts Key struct to a []*agent.AddedKey. All elements
// of the []*agent.AddedKey slice need to be loaded into the agent!
func AsAgentKeys(sshCert *ssh.Certificate, privKey []byte) ([]agent.AddedKey, error) {
	// unmarshal private key bytes into a *rsa.PrivateKey
	privateKey, err := ssh.ParseRawPrivateKey(privKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// put a teleport identifier along with the teleport user into the comment field
	comment := fmt.Sprintf("teleport:%v", sshCert.KeyId)

	// On Windows, return the certificate with the private key embedded.
	if runtime.GOOS == constants.WindowsOS {
		return []agent.AddedKey{
			{
				PrivateKey:       privateKey,
				Certificate:      sshCert,
				Comment:          comment,
				LifetimeSecs:     0,
				ConfirmBeforeUse: false,
			},
		}, nil
	}

	// On Unix, return the certificate (with embedded private key) as well as
	// a private key.
	//
	// This is done because OpenSSH clients older than OpenSSH 7.3/7.3p1
	// (2016-08-01) have a bug in how they use certificates that have been loaded
	// in an agent. Specifically when you add a certificate to an agent, you can't
	// just embed the private key within the certificate, you have to add the
	// certificate and private key to the agent separately. Teleport works around
	// this behavior to ensure OpenSSH interoperability.
	//
	// For more details see the following: https://bugzilla.mindrot.org/show_bug.cgi?id=2550
	// WARNING: callers expect the returned slice to be __exactly as it is__
	return []agent.AddedKey{
		{
			PrivateKey:       privateKey,
			Certificate:      sshCert,
			Comment:          comment,
			LifetimeSecs:     0,
			ConfirmBeforeUse: false,
		},
		{
			PrivateKey:       privateKey,
			Certificate:      nil,
			Comment:          comment,
			LifetimeSecs:     0,
			ConfirmBeforeUse: false,
		},
	}, nil
}

// HostKeyCallback returns an ssh.HostKeyCallback that validates host
// keys/certs against SSH CAs in the Key.
//
// If not CAs are present in the Key, the returned ssh.HostKeyCallback is nil.
// This causes golang.org/x/crypto/ssh to prompt the user to verify host key
// fingerprint (same as OpenSSH does for an unknown host).
func HostKeyCallback(caCerts [][]byte, withHostKeyFallback bool) (ssh.HostKeyCallback, error) {
	trustedKeys, err := ParseKnownHosts(caCerts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// No CAs are provided, return a nil callback which will prompt the user
	// for trust.
	if len(trustedKeys) == 0 {
		return nil, nil
	}

	callbackConfig := HostKeyCallbackConfig{
		GetHostCheckers: func() ([]ssh.PublicKey, error) {
			return trustedKeys, nil
		},
	}

	if withHostKeyFallback {
		callbackConfig.HostKeyFallback = hostKeyFallbackFunc(trustedKeys)
	}

	callback, err := NewHostKeyCallback(callbackConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return callback, nil
}

func hostKeyFallbackFunc(knownHosts []ssh.PublicKey) func(hostname string, remote net.Addr, key ssh.PublicKey) error {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		for _, knownHost := range knownHosts {
			if KeysEqual(key, knownHost) {
				return nil
			}
		}
		return trace.AccessDenied("host %v presented a public key instead of a host certificate which isn't among known hosts", hostname)
	}
}

// KeysEqual is constant time compare of the keys to avoid timing attacks
func KeysEqual(ak, bk ssh.PublicKey) bool {
	a := ssh.Marshal(ak)
	b := ssh.Marshal(bk)
	return (len(a) == len(b) && subtle.ConstantTimeCompare(a, b) == 1)
}

// ConvertToPPK takes a regular RSA-formatted keypair and converts it into the PPK file format used by the PuTTY SSH client.
// The file format is described here: https://the.earth.li/~sgtatham/putty/0.76/htmldoc/AppendixC.html#ppk
func ConvertToPPK(priv []byte, pub []byte) ([]byte, error) {
	// decode the private key from PEM format and extract the exponents
	privateKeyPemBlock, rest := pem.Decode(priv)
	if len(rest) > 0 {
		return nil, trace.Wrap(fmt.Errorf("failed to decode private key, %v bytes left over", len(rest)))
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(privateKeyPemBlock.Bytes)
	if err != nil {
		return nil, trace.Wrap(fmt.Errorf("failed to parse private key: %T", err))
	}

	// https://the.earth.li/~sgtatham/putty/0.76/htmldoc/AppendixC.html#ppk
	// RSA keys are stored using an algorithm-name of ‘ssh-rsa’. (Keys stored like this are also used by the updated RSA signature schemes that use
	// hashes other than SHA-1. The public key data has already provided the key modulus and the public encoding exponent. The private data stores:
	// mpint: the private decoding exponent of the key.
	// mpint: one prime factor p of the key.
	// mpint: the other prime factor q of the key. (RSA keys stored in this format are expected to have exactly two prime factors.)
	// mpint: the multiplicative inverse of q modulo p.
	ppkPrivateKey := new(bytes.Buffer)

	// mpint: the private decoding exponent of the key.
	// this is known as 'D'
	mpintD, err := getRFC4251Mpint(privateKey.D)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = binary.Write(ppkPrivateKey, binary.BigEndian, mpintD)
	if err != nil {
		return nil, trace.Wrap(fmt.Errorf("failed to write D to buffer: %T", err))
	}

	// mpint: one prime factor p of the key.
	// this is known as 'P'
	// the RSA standard dictates that P > Q
	// for some reason what PuTTY names 'P' is Primes[1] to Go, and what PuTTY names 'Q' is Primes[0] to Go
	P := privateKey.Primes[1]
	mpintP, err := getRFC4251Mpint(P)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = binary.Write(ppkPrivateKey, binary.BigEndian, mpintP)
	if err != nil {
		return nil, trace.Wrap(fmt.Errorf("failed to write P to buffer: %T", err))
	}

	// mpint: the other prime factor q of the key. (RSA keys stored in this format are expected to have exactly two prime factors.)
	// this is known as 'Q'
	Q := privateKey.Primes[0]
	mpintQ, err := getRFC4251Mpint(Q)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = binary.Write(ppkPrivateKey, binary.BigEndian, mpintQ)
	if err != nil {
		return nil, trace.Wrap(fmt.Errorf("failed to write Q to buffer: %T", err))
	}

	// mpint: the multiplicative inverse of q modulo p.
	// this is known as 'iqmp'
	iqmp := new(big.Int).ModInverse(Q, P)
	mpintIQMP, err := getRFC4251Mpint(iqmp)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = binary.Write(ppkPrivateKey, binary.BigEndian, mpintIQMP)
	if err != nil {
		return nil, trace.Wrap(fmt.Errorf("failed to write iqmp to buffer: %T", err))
	}

	// now we need to base64-encode the PPK-formatted private key which is made up of the above values
	ppkPrivateKeyBase64 := make([]byte, base64.StdEncoding.EncodedLen(ppkPrivateKey.Len()))
	base64.StdEncoding.Encode(ppkPrivateKeyBase64, ppkPrivateKey.Bytes())

	// build Teleport public key path
	// fortunately, this is the one thing that's in exactly the same format that the PPK file uses, so we just copy it verbatim
	teleportPublicKey := string(pub)
	// remove ssh-rsa from beginning of string if present
	if !strings.HasPrefix(teleportPublicKey, "ssh-rsa") {
		return nil, trace.Wrap(fmt.Errorf("pub does not appear to be an ssh-rsa public key: %T", err))
	}
	teleportPublicKey = strings.TrimSuffix(strings.TrimPrefix(teleportPublicKey, "ssh-rsa "), "\n")
	// we use the trimmed public key from here on out
	teleportPublicKeyBytes := []byte(teleportPublicKey)

	// the PPK file contains an anti-tampering MAC which is made up of various values which appear in the file.
	// copied from Section C.3 of https://the.earth.li/~sgtatham/putty/0.76/htmldoc/AppendixC.html#ppk:
	// hex-mac-data is a hexadecimal-encoded value, 64 digits long (i.e. 32 bytes), generated using the HMAC-SHA-256 algorithm with the following binary data as input:
	// string: the algorithm-name header field.
	// string: the encryption-type header field.
	// string: the key-comment-string header field.
	// string: the binary public key data, as decoded from the base64 lines after the 'Public-Lines' header.
	// string: the plaintext of the binary private key data, as decoded from the base64 lines after the 'Private-Lines' header.

	// these values are also used in the MAC generation, so we declare them as variables
	keyType := "ssh-rsa"
	encryptionType := "none"
	// TODO(gus): is there a way we can get the proxy/user pair name in here?
	// annoyingly this has to be done at generation time because the comment is part of the MAC
	fileComment := "teleport-generated-ppk"

	// create a buffer to hold the elements needed to generate the MAC
	macInput := new(bytes.Buffer)

	// string: the algorithm-name header field.
	macKeyType, err := getRFC4251String([]byte(keyType))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = binary.Write(macInput, binary.LittleEndian, macKeyType)
	if err != nil {
		return nil, trace.Wrap(fmt.Errorf("cannot write macKeyType to buffer: %T", err))
	}

	// string: the encryption-type header field.
	macEncryptionType, err := getRFC4251String([]byte(encryptionType))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = binary.Write(macInput, binary.BigEndian, macEncryptionType)
	if err != nil {
		return nil, trace.Wrap(fmt.Errorf("cannot write macEncryptionType to buffer: %T", err))
	}

	// string: the key-comment-string header field.
	macComment, err := getRFC4251String([]byte(fileComment))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = binary.Write(macInput, binary.BigEndian, macComment)
	if err != nil {
		return nil, trace.Wrap(fmt.Errorf("cannot write macComment to buffer: %T", err))
	}

	// base64-decode the Teleport public key, as we need its binary representation to generate the MAC
	teleportPublicKeyDecoded := make([]byte, base64.StdEncoding.EncodedLen(len(teleportPublicKeyBytes)))
	teleportPublicKeyByteCount, err := base64.StdEncoding.Decode(teleportPublicKeyDecoded, teleportPublicKeyBytes)
	publicKeyData := make([]byte, teleportPublicKeyByteCount)
	copy(publicKeyData, teleportPublicKeyDecoded)
	if err != nil {
		return nil, trace.Wrap(fmt.Errorf("could not base64-decode public key: %T, got %v bytes successfully", err, teleportPublicKeyByteCount))
	}
	// append the decoded public key bytes to the MAC buffer
	macPublicKeyData, err := getRFC4251String(publicKeyData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = binary.Write(macInput, binary.BigEndian, macPublicKeyData)
	if err != nil {
		return nil, trace.Wrap(fmt.Errorf("cannot write macPublicKeyData to buffer: %T", err))
	}

	// append our PPK-formatted private key bytes to the MAC buffer
	macPrivateKeyData, err := getRFC4251String(ppkPrivateKey.Bytes())
	if err != nil {
		return nil, err
	}
	err = binary.Write(macInput, binary.BigEndian, macPrivateKeyData)
	if err != nil {
		return nil, trace.Wrap(fmt.Errorf("cannot write macPrivateKeyData to buffer: %T", err))
	}

	// as per the PPK spec, the key for the MAC is blank when the PPK file is unencrypted.
	// therefore, the key is a zero-length byte slice.
	hmacHash := hmac.New(sha256.New, []byte{})
	// generate the MAC using HMAC-SHA-256
	hmacHash.Write(macInput.Bytes())
	macString := hex.EncodeToString(hmacHash.Sum(nil))

	// build the string-formatted output PPK file
	var ppk strings.Builder
	ppk.WriteString(fmt.Sprintf("PuTTY-User-Key-File-3: %v\n", keyType))
	ppk.WriteString(fmt.Sprintf("Encryption: %v\n", encryptionType))
	ppk.WriteString(fmt.Sprintf("Comment: %v\n", fileComment))
	// chunk the Teleport-formatted public key into 64-character length lines
	chunkedPublicKey := chunkString(teleportPublicKey, 64)
	ppk.WriteString(fmt.Sprintf("Public-Lines: %v\n", len(chunkedPublicKey)))
	for _, r := range chunkedPublicKey {
		ppk.WriteString(fmt.Sprintf("%s\n", r))
	}
	// chunk the PPK-formatted private key into 64-character length lines
	chunkedPrivateKey := chunkString(string(ppkPrivateKeyBase64), 64)
	ppk.WriteString(fmt.Sprintf("Private-Lines: %v\n", len(chunkedPrivateKey)))
	for _, r := range chunkedPrivateKey {
		ppk.WriteString(fmt.Sprintf("%s\n", r))
	}
	ppk.WriteString(fmt.Sprintf("Private-MAC: %v\n", macString))

	// convert string to bytes and return
	return []byte(ppk.String()), nil
}

// chunkString converts a string into a []string with chunks of size chunkSize
// used to split base64-encoded strings across multiple lines with an even width
func chunkString(s string, chunkSize int) []string {
	if len(s) == 0 {
		return nil
	}
	if chunkSize >= len(s) {
		return []string{s}
	}
	chunks := make([]string, 0, (len(s)-1)/chunkSize+1)
	currentLen := 0
	currentStart := 0
	for i := range s {
		if currentLen == chunkSize {
			chunks = append(chunks, s[currentStart:i])
			currentLen = 0
			currentStart = i
		}
		currentLen++
	}
	chunks = append(chunks, s[currentStart:])
	return chunks
}

// getRFC4251Mpint returns a stream of bytes representing a mixed-precision integer (a big.Int in Go)
// prepended with a big-endian uint32 expressing the length of the data following.
// This is the 'mpint' format in RFC4251 Section 5 (https://datatracker.ietf.org/doc/html/rfc4251#section-5)
func getRFC4251Mpint(n *big.Int) ([]byte, error) {
	buf := new(bytes.Buffer)
	b := n.Bytes()
	// RFC4251: If the most significant bit would be set for a positive number, the number MUST be preceded by a zero byte.
	if b[0]&0x80 > 0 {
		b = append([]byte{0}, b...)
	}
	// write a uint32 with the length of the byte stream to the buffer
	err := binary.Write(buf, binary.BigEndian, uint32(len(b)))
	if err != nil {
		return nil, trace.Wrap(fmt.Errorf("failed to write length uint32 to buffer: %T", err))
	}
	// write the byte stream representing of the rest of the integer to the buffer
	err = binary.Write(buf, binary.BigEndian, b)
	if err != nil {
		return nil, trace.Wrap(fmt.Errorf("failed to write data to buffer: %T", err))
	}
	return buf.Bytes(), nil
}

// getRFC4251String returns a stream of bytes representing a string prepended with a big-endian unit32
// expressing the length of the data following.
// This is the 'string' format in RFC4251 Section 5 (https://datatracker.ietf.org/doc/html/rfc4251#section-5)
func getRFC4251String(data []byte) ([]byte, error) {
	buf := new(bytes.Buffer)
	// write a uint32 with the length of the byte stream to the buffer
	err := binary.Write(buf, binary.BigEndian, uint32(len(data)))
	if err != nil {
		return nil, trace.Wrap(fmt.Errorf("failed to write length uint32 to buffer: %T", err))
	}
	// write the byte stream representing of the rest of the data to the buffer
	for _, v := range data {
		err := binary.Write(buf, binary.BigEndian, v)
		if err != nil {
			return nil, trace.Wrap(fmt.Errorf("failed to write data to buffer: %T", err))
		}
	}
	return buf.Bytes(), nil
}
