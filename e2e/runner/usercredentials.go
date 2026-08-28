/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"log/slog"
	"math/big"

	"golang.org/x/crypto/bcrypt"
)

type credentials struct {
	password              string
	passwordHashBase64    string
	credentialIDBase64    string
	publicKeyCBORBase64   string
	privateKeyPKCS8Base64 string

	// clientIP is the X-Forwarded-For this user's setup login claims, giving it its own rate limiter bucket.
	clientIP string
}

// Addresses have to be IPv6: the proxy signs a PROXY header pairing this source with the destination of the
// browser's own connection, which is ::1 because the tests reach the proxy over localhost, and signing rejects
// a version mismatch. Playwright claims fd00:e2e:2:: for per-test addresses.
const userIPPrefix = "fd00:e2e:1"

// assignClientIP maps a bootstrap list position to a unique address.
func assignClientIP(index int) string {
	return fmt.Sprintf("%s:%x::%x", userIPPrefix, index/0x10000, index%0x10000)
}

// generateUserCredentials creates a fresh password, bcrypt hash, and ECDSA key pair
// to be used as the bootstrapped test user's credentials.
func generateUserCredentials() (*credentials, error) {
	// generate password
	password, err := randomAlphanumeric(24)
	if err != nil {
		return nil, fmt.Errorf("generating password: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	// generate webauthn
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating ECDSA key: %w", err)
	}

	pkcs8, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("marshaling private key: %w", err)
	}

	pub, err := privateKey.PublicKey.Bytes()
	if err != nil {
		return nil, fmt.Errorf("encoding public key: %w", err)
	}

	// The public key is the uncompressed point (0x04 || X || Y), with each
	// coordinate zero-padded to 32 bytes.
	x, y := pub[1:33], pub[33:65]

	pubCBOR := encodeEC2PublicKeyCBOR(x, y)

	credID := make([]byte, 32)
	if _, err := rand.Read(credID); err != nil {
		return nil, fmt.Errorf("generating credential ID: %w", err)
	}

	creds := &credentials{
		password:              password,
		passwordHashBase64:    base64.StdEncoding.EncodeToString(hash),
		credentialIDBase64:    base64.StdEncoding.EncodeToString(credID),
		publicKeyCBORBase64:   base64.StdEncoding.EncodeToString(pubCBOR),
		privateKeyPKCS8Base64: base64.StdEncoding.EncodeToString(pkcs8),
	}

	slog.DebugContext(context.Background(), "generated per-run credentials",
		"credential_id", creds.credentialIDBase64,
	)

	return creds, nil
}

// encodeEC2PublicKeyCBOR builds the 77-byte COSE key encoding for an
// EC2 P-256 public key. The structure is fixed so we construct the bytes
// directly rather than pulling in a CBOR library. x and y must each be the
// 32-byte big-endian coordinate.
func encodeEC2PublicKeyCBOR(x, y []byte) []byte {
	buf := make([]byte, 0, 77)

	buf = append(buf, 0xa5)       // map(5)
	buf = append(buf, 0x01)       // key: 1 (kty)
	buf = append(buf, 0x02)       // value: 2 (EC2)
	buf = append(buf, 0x03)       // key: 3 (alg)
	buf = append(buf, 0x26)       // value: -7 (ES256)
	buf = append(buf, 0x20)       // key: -1 (crv)
	buf = append(buf, 0x01)       // value: 1 (P-256)
	buf = append(buf, 0x21)       // key: -2 (x)
	buf = append(buf, 0x58, 0x20) // bstr(32)
	buf = append(buf, x...)
	buf = append(buf, 0x22)       // key: -3 (y)
	buf = append(buf, 0x58, 0x20) // bstr(32)
	buf = append(buf, y...)

	return buf
}

func randomAlphanumeric(n int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	max := big.NewInt(int64(len(alphabet)))
	b := make([]byte, n)

	for i := range b {
		idx, err := rand.Int(rand.Reader, max)

		if err != nil {
			return "", err
		}

		b[i] = alphabet[idx.Int64()]
	}

	return string(b), nil
}
