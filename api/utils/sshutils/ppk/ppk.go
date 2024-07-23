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
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
)

// ConvertToPPK takes a regular RSA-formatted keypair and converts it into the PPK file format used by the PuTTY SSH client.
// The file format is described here: https://the.earth.li/~sgtatham/putty/0.76/htmldoc/AppendixC.html#ppk
//
// TODO(nklaassen): support Ed25519 and ECDSA keys. The file format supports it,
// we just don't support writing them here.
func ConvertToPPK(privateKey *rsa.PrivateKey, pub []byte) ([]byte, error) {
	// https://the.earth.li/~sgtatham/putty/0.76/htmldoc/AppendixC.html#ppk
	// RSA keys are stored using an algorithm-name of 'ssh-rsa'. (Keys stored like this are also used by the updated RSA signature schemes that use
	// hashes other than SHA-1. The public key data has already provided the key modulus and the public encoding exponent. The private data stores:
	// mpint: the private decoding exponent of the key.
	// mpint: one prime factor p of the key.
	// mpint: the other prime factor q of the key. (RSA keys stored in this format are expected to have exactly two prime factors.)
	// mpint: the multiplicative inverse of q modulo p.
	ppkPrivateKey := new(bytes.Buffer)

	// mpint: the private decoding exponent of the key.
	// this is known as 'D'
	binary.Write(ppkPrivateKey, binary.BigEndian, getRFC4251Mpint(privateKey.D))

	// mpint: one prime factor p of the key.
	// this is known as 'P'
	// the RSA standard dictates that P > Q
	// for some reason what PuTTY names 'P' is Primes[1] to Go, and what PuTTY names 'Q' is Primes[0] to Go
	P, Q := privateKey.Primes[1], privateKey.Primes[0]
	binary.Write(ppkPrivateKey, binary.BigEndian, getRFC4251Mpint(P))

	// mpint: the other prime factor q of the key. (RSA keys stored in this format are expected to have exactly two prime factors.)
	// this is known as 'Q'
	binary.Write(ppkPrivateKey, binary.BigEndian, getRFC4251Mpint(Q))

	// mpint: the multiplicative inverse of q modulo p.
	// this is known as 'iqmp'
	iqmp := new(big.Int).ModInverse(Q, P)
	binary.Write(ppkPrivateKey, binary.BigEndian, getRFC4251Mpint(iqmp))

	// now we need to base64-encode the PPK-formatted private key which is made up of the above values
	ppkPrivateKeyBase64 := make([]byte, base64.StdEncoding.EncodedLen(ppkPrivateKey.Len()))
	base64.StdEncoding.Encode(ppkPrivateKeyBase64, ppkPrivateKey.Bytes())

	// read Teleport public key
	// fortunately, this is the one thing that's in exactly the same format that the PPK file uses, so we can just copy it verbatim
	// remove ssh-rsa plus additional space from beginning of string if present
	if !bytes.HasPrefix(pub, []byte(constants.SSHRSAType+" ")) {
		return nil, trace.BadParameter("pub does not appear to be an ssh-rsa public key")
	}
	pub = bytes.TrimSuffix(bytes.TrimPrefix(pub, []byte(constants.SSHRSAType+" ")), []byte("\n"))

	// the PPK file contains an anti-tampering MAC which is made up of various values which appear in the file.
	// copied from Section C.3 of https://the.earth.li/~sgtatham/putty/0.76/htmldoc/AppendixC.html#ppk:
	// hex-mac-data is a hexadecimal-encoded value, 64 digits long (i.e. 32 bytes), generated using the HMAC-SHA-256 algorithm with the following binary data as input:
	// string: the algorithm-name header field.
	// string: the encryption-type header field.
	// string: the key-comment-string header field.
	// string: the binary public key data, as decoded from the base64 lines after the 'Public-Lines' header.
	// string: the plaintext of the binary private key data, as decoded from the base64 lines after the 'Private-Lines' header.

	// these values are also used in the MAC generation, so we declare them as variables
	keyType := constants.SSHRSAType
	encryptionType := "none"
	// as work for the future, it'd be nice to get the proxy/user pair name in here to make the name more
	// of a unique identifier. this has to be done at generation time because the comment is part of the MAC
	fileComment := "teleport-generated-ppk"

	// string: the algorithm-name header field.
	macKeyType := getRFC4251String([]byte(keyType))
	// create a buffer to hold the elements needed to generate the MAC
	macInput := new(bytes.Buffer)
	binary.Write(macInput, binary.LittleEndian, macKeyType)

	// string: the encryption-type header field.
	macEncryptionType := getRFC4251String([]byte(encryptionType))
	binary.Write(macInput, binary.BigEndian, macEncryptionType)

	// string: the key-comment-string header field.
	macComment := getRFC4251String([]byte(fileComment))
	binary.Write(macInput, binary.BigEndian, macComment)

	// base64-decode the Teleport public key, as we need its binary representation to generate the MAC
	decoded := make([]byte, base64.StdEncoding.EncodedLen(len(pub)))
	n, err := base64.StdEncoding.Decode(decoded, pub)
	if err != nil {
		return nil, trace.Errorf("could not base64-decode public key: %v, got %v bytes successfully", err, n)
	}
	decoded = decoded[:n]
	// append the decoded public key bytes to the MAC buffer
	macPublicKeyData := getRFC4251String(decoded)
	binary.Write(macInput, binary.BigEndian, macPublicKeyData)

	// append our PPK-formatted private key bytes to the MAC buffer
	macPrivateKeyData := getRFC4251String(ppkPrivateKey.Bytes())
	binary.Write(macInput, binary.BigEndian, macPrivateKeyData)

	// as per the PPK spec, the key for the MAC is blank when the PPK file is unencrypted.
	// therefore, the key is a zero-length byte slice.
	hmacHash := hmac.New(sha256.New, []byte{})
	// generate the MAC using HMAC-SHA-256
	hmacHash.Write(macInput.Bytes())
	macString := hex.EncodeToString(hmacHash.Sum(nil))

	// build the string-formatted output PPK file
	ppk := new(bytes.Buffer)
	fmt.Fprintf(ppk, "PuTTY-User-Key-File-3: %v\n", keyType)
	fmt.Fprintf(ppk, "Encryption: %v\n", encryptionType)
	fmt.Fprintf(ppk, "Comment: %v\n", fileComment)
	// chunk the Teleport-formatted public key into 64-character length lines
	chunkedPublicKey := chunk(string(pub), 64)
	fmt.Fprintf(ppk, "Public-Lines: %v\n", len(chunkedPublicKey))
	for _, r := range chunkedPublicKey {
		fmt.Fprintf(ppk, "%s\n", r)
	}
	// chunk the PPK-formatted private key into 64-character length lines
	chunkedPrivateKey := chunk(string(ppkPrivateKeyBase64), 64)
	fmt.Fprintf(ppk, "Private-Lines: %v\n", len(chunkedPrivateKey))
	for _, r := range chunkedPrivateKey {
		fmt.Fprintf(ppk, "%s\n", r)
	}
	fmt.Fprintf(ppk, "Private-MAC: %v\n", macString)

	return ppk.Bytes(), nil
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

// getRFC4251Mpint returns a stream of bytes representing a mixed-precision integer (a big.Int in Go)
// prepended with a big-endian uint32 expressing the length of the data following.
// This is the 'mpint' format in RFC4251 Section 5 (https://datatracker.ietf.org/doc/html/rfc4251#section-5)
func getRFC4251Mpint(n *big.Int) []byte {
	buf := new(bytes.Buffer)
	b := n.Bytes()
	// RFC4251: If the most significant bit would be set for a positive number, the number MUST be preceded by a zero byte.
	if b[0]&0x80 > 0 {
		b = append([]byte{0}, b...)
	}
	// write a uint32 with the length of the byte stream to the buffer
	binary.Write(buf, binary.BigEndian, uint32(len(b)))
	// write the byte stream representing of the rest of the integer to the buffer
	binary.Write(buf, binary.BigEndian, b)
	return buf.Bytes()
}

// getRFC4251String returns a stream of bytes representing a string prepended with a big-endian unit32
// expressing the length of the data following.
// This is the 'string' format in RFC4251 Section 5 (https://datatracker.ietf.org/doc/html/rfc4251#section-5)
func getRFC4251String(data []byte) []byte {
	buf := new(bytes.Buffer)
	// write a uint32 with the length of the byte stream to the buffer
	binary.Write(buf, binary.BigEndian, uint32(len(data)))
	// write the byte stream representing of the rest of the data to the buffer
	for _, v := range data {
		binary.Write(buf, binary.BigEndian, v)
	}
	return buf.Bytes()
}
