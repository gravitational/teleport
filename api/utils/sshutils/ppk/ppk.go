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

// Package ppk provides functions implementing conversion between Teleport's native RSA
// keypairs and PuTTY's PPK format. It also provides functions for working with RFC4251-formatted
// mpints and strings.
package ppk

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

const (
	encryptionType = "none"
	// As work for the future, it'd be nice to get the proxy/user pair name in here to make the name more
	// of a unique identifier. this has to be done at generation time because the comment is part of the MAC
	fileComment = "teleport-generated-ppk"
)

// ConvertToPPK takes a regular SSH keypair and converts it into the PPK file format used by the PuTTY SSH client.
// The file format is described here: https://the.earth.li/~sgtatham/putty/0.76/htmldoc/AppendixC.html#ppk
func ConvertToPPK(privateKey crypto.Signer, pub ssh.PublicKey) ([]byte, error) {
	var ppkPrivateKey bytes.Buffer
	if err := writePrivateKey(&ppkPrivateKey, privateKey); err != nil {
		return nil, trace.Wrap(err)
	}
	ppkPrivateKeyBase64 := base64.StdEncoding.EncodeToString(ppkPrivateKey.Bytes())
	ppkPublicKeyBase64 := base64.StdEncoding.EncodeToString(pub.Marshal())

	// Compute the anti-tampering MAC.
	macString, err := computeMAC(pub, ppkPrivateKey.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Build the string-formatted output PPK file.
	ppk := new(bytes.Buffer)
	fmt.Fprintf(ppk, "PuTTY-User-Key-File-3: %v\n", pub.Type())
	fmt.Fprintf(ppk, "Encryption: %v\n", encryptionType)
	fmt.Fprintf(ppk, "Comment: %v\n", fileComment)
	// Chunk the base64-encoded public key into 64-character length lines.
	chunkedPublicKey := chunk(ppkPublicKeyBase64, 64)
	fmt.Fprintf(ppk, "Public-Lines: %v\n", len(chunkedPublicKey))
	for _, r := range chunkedPublicKey {
		fmt.Fprintf(ppk, "%s\n", r)
	}
	// Chunk the PPK-formatted private key into 64-character length lines.
	chunkedPrivateKey := chunk(ppkPrivateKeyBase64, 64)
	fmt.Fprintf(ppk, "Private-Lines: %v\n", len(chunkedPrivateKey))
	for _, r := range chunkedPrivateKey {
		fmt.Fprintf(ppk, "%s\n", r)
	}
	fmt.Fprintf(ppk, "Private-MAC: %v\n", macString)

	return ppk.Bytes(), nil
}

// computeMAC computes an anti-tampering MAC which is made up of various values which appear in the PPK file.
// Copied from Section C.2 of https://the.earth.li/~sgtatham/putty/0.76/htmldoc/AppendixC.html#ppk:
// hex-mac-data is a hexadecimal-encoded value, 64 digits long (i.e. 32 bytes), generated using the HMAC-SHA-256 algorithm with the following binary data as input:
// string: the algorithm-name header field.
// string: the encryption-type header field.
// string: the key-comment-string header field.
// string: the binary public key data, as decoded from the base64 lines after the 'Public-Lines' header.
// string: the plaintext of the binary private key data, as decoded from the base64 lines after the 'Private-Lines' header.
func computeMAC(pub ssh.PublicKey, rawPrivateKey []byte) (string, error) {
	// Generate the MAC using HMAC-SHA-256. As per the PPK spec, the key for the
	// MAC is blank when the PPK file is unencrypted.
	var hmacKey []byte
	hmacHash := hmac.New(sha256.New, hmacKey)
	if err := writeRFC4251Strings(hmacHash,
		[]byte(pub.Type()),     // the algorithm-name header field
		[]byte(encryptionType), // the encryption-type header field
		[]byte(fileComment),    // the key-comment-string header field
		pub.Marshal(),          // the binary public-key data
		rawPrivateKey,          // the plaintext of the binary private key data
	); err != nil {
		return "", trace.Wrap(err)
	}
	return hex.EncodeToString(hmacHash.Sum(nil)), nil
}

func writePrivateKey(w io.Writer, signer crypto.Signer) error {
	switch k := signer.(type) {
	case *rsa.PrivateKey:
		return trace.Wrap(writeRSAPrivateKey(w, k))
	case *ecdsa.PrivateKey:
		return trace.Wrap(writeECDSAPrivateKey(w, k))
	case ed25519.PrivateKey:
		return trace.Wrap(writeEd25519PrivateKey(w, k))
	}
	return trace.BadParameter("unsupported private key type %T", signer)
}

func writeRSAPrivateKey(w io.Writer, privateKey *rsa.PrivateKey) error {
	// https://the.earth.li/~sgtatham/putty/0.76/htmldoc/AppendixC.html#ppk
	// RSA keys are stored using an algorithm-name of 'ssh-rsa'. (Keys stored like this are also used by the updated RSA signature schemes that use
	// hashes other than SHA-1. The public key data has already provided the key modulus and the public encoding exponent. The private data stores:
	// mpint: the private decoding exponent of the key.
	// mpint: one prime factor p of the key.
	// mpint: the other prime factor q of the key. (RSA keys stored in this format are expected to have exactly two prime factors.)
	// mpint: the multiplicative inverse of q modulo p.

	// For some reason what PuTTY names 'P' is Primes[1] to Go, and what PuTTY
	// names 'Q' is Primes[0] to Go. RSA keys stored in this format are
	// expected to have exactly two prime factors.
	P, Q := privateKey.Primes[1], privateKey.Primes[0]
	// The multiplicative inverse of q modulo p.
	iqmp := new(big.Int).ModInverse(Q, P)
	return trace.Wrap(writeRFC4251Mpints(w, privateKey.D, P, Q, iqmp))
}

func writeECDSAPrivateKey(w io.Writer, privateKey *ecdsa.PrivateKey) error {
	// https://the.earth.li/~sgtatham/putty/0.76/htmldoc/AppendixC.html#ppk
	// NIST elliptic-curve keys are stored using one of the following
	// algorithm-name values, each corresponding to a different elliptic curve
	// and key size:
	// - ‘ecdsa-sha2-nistp256’
	// - ‘ecdsa-sha2-nistp384’
	// - ‘ecdsa-sha2-nistp521’
	// The public key data has already provided the public elliptic curve point. The private key stores:
	// mpint: the private exponent, which is the discrete log of the public point.
	//
	// crypto/ecdsa calls this D.
	return trace.Wrap(writeRFC4251Mpint(w, privateKey.D))
}

func writeEd25519PrivateKey(w io.Writer, privateKey ed25519.PrivateKey) error {
	// https://the.earth.li/~sgtatham/putty/0.76/htmldoc/AppendixC.html#ppk
	// EdDSA elliptic-curve keys are stored using one of the following
	// algorithm-name values, each corresponding to a different elliptic curve
	// and key size:
	// - ‘ssh-ed25519’
	// - ‘ssh-ed448’
	// The public key data has already provided the public elliptic curve point. The private key stores:
	// mpint: the private exponent, which is the discrete log of the public point.
	//
	// crypto/ed25519 calls the private exponent the seed.
	return trace.Wrap(writeRFC4251Mpint(w, new(big.Int).SetBytes(privateKey.Seed())))
}

func writeRFC4251Mpints(w io.Writer, ints ...*big.Int) error {
	for _, n := range ints {
		if err := writeRFC4251Mpint(w, n); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// writeRFC4251Mpint writes a stream of bytes representing a big-endian
// mixed-precision integer (a big.Int in Go) in the 'mpint' format described in
// RFC4251 Section 5 (https://datatracker.ietf.org/doc/html/rfc4251#section-5)
func writeRFC4251Mpint(w io.Writer, n *big.Int) error {
	b := n.Bytes()
	// RFC4251: If the most significant bit would be set for a positive number, the number MUST be preceded by a zero byte.
	if n.Sign() == 1 && b[0]&0x80 != 0 {
		b = append([]byte{0}, b...)
	}
	return trace.Wrap(writeRFC4251String(w, b))
}

func writeRFC4251Strings(w io.Writer, strs ...[]byte) error {
	for _, s := range strs {
		if err := writeRFC4251String(w, s); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// writeRFC4251String writes a stream of bytes prepended with a big-endian
// uint32 expressing the length of the data following.
// This is the 'string' format in RFC4251 Section 5 (https://datatracker.ietf.org/doc/html/rfc4251#section-5)
func writeRFC4251String(w io.Writer, s []byte) error {
	// Write a uint32 with the length of the byte stream to the buffer.
	if err := binary.Write(w, binary.BigEndian, uint32(len(s))); err != nil {
		return trace.Wrap(err)
	}
	// Write the byte stream representing of the rest of the data to the buffer.
	if _, err := io.Copy(w, bytes.NewReader(s)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// chunk converts a string into a []string with chunks of size chunkSize;
// used to split base64-encoded strings across multiple lines with an even width.
// note: this function operates on Unicode code points rather than bytes, therefore
// using it with multi-byte characters will result in unevenly chunked strings.
// it's intended usage is only for chunking base64-encoded strings.
func chunk(s string, size int) []string {
	var chunks []string
	for b := []byte(s); len(b) > 0; {
		n := size
		if n > len(b) {
			n = len(b)
		}
		chunks = append(chunks, string(b[:n]))
		b = b[n:]
	}
	return chunks
}
