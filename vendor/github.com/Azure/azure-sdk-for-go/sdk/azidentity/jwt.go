// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azidentity

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/internal/uuid"
)

// headerJWT type contains the fields necessary to create a JSON Web Token including the x5t field which must contain a x.509 certificate thumbprint
type headerJWT struct {
	Typ string   `json:"typ"`
	Alg string   `json:"alg"`
	X5t string   `json:"x5t"`
	X5c []string `json:"x5c,omitempty"`
}

// payloadJWT type contains all fields that are necessary when creating a JSON Web Token payload section
type payloadJWT struct {
	JTI string `json:"jti"`
	AUD string `json:"aud"`
	ISS string `json:"iss"`
	SUB string `json:"sub"`
	NBF int64  `json:"nbf"`
	EXP int64  `json:"exp"`
}

// createClientAssertionJWT build the JWT header, payload and signature,
// then returns a string for the JWT assertion
func createClientAssertionJWT(clientID string, audience string, cert *certContents, sendCertificateChain bool) (string, error) {
	headerData := headerJWT{
		Typ: "JWT",
		Alg: "RS256",
		X5t: base64.RawURLEncoding.EncodeToString(cert.fp),
	}
	if sendCertificateChain {
		headerData.X5c = cert.publicCertificates
	}

	headerJSON, err := json.Marshal(headerData)
	if err != nil {
		return "", fmt.Errorf("marshal headerJWT: %w", err)
	}
	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	jti, err := uuid.New()
	if err != nil {
		return "", err
	}
	payloadData := payloadJWT{
		JTI: jti.String(),
		AUD: audience,
		ISS: clientID,
		SUB: clientID,
		NBF: time.Now().Unix(),
		EXP: time.Now().Add(30 * time.Minute).Unix(),
	}

	payloadJSON, err := json.Marshal(payloadData)
	if err != nil {
		return "", fmt.Errorf("marshal payloadJWT: %w", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	result := header + "." + payload
	hashed := []byte(result)
	hashedSum := sha256.Sum256(hashed)
	cryptoRand := rand.Reader

	signed, err := rsa.SignPKCS1v15(cryptoRand, cert.pk, crypto.SHA256, hashedSum[:])
	if err != nil {
		return "", err
	}

	signature := base64.RawURLEncoding.EncodeToString(signed)

	return result + "." + signature, nil
}
