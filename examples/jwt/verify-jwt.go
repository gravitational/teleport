/*
Copyright 2020 Gravitational, Inc.

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

package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
)

// jwk is a JSON Web Key, described in detail in RFC 7517.
type jwk struct {
	// KeyType is the type of asymmetric key used.
	KeyType string `json:"kty"`
	// Algorithm used to sign.
	Algorithm string `json:"alg"`

	// N is the modulus of the public key.
	N string `json:"n"`
	// E is the exponent of the public key.
	E string `json:"e"`

	// Curve identifies the cryptographic curve used with an ECDSA public key.
	Curve string `json:"crv,omitempty"`
	// X is the x coordinate parameter of an ECDSA public key.
	X string `json:"x,omitempty"`
	// Y is the y coordinate parameter of an ECDSA public key.
	Y string `json:"y,omitempty"`
}

// jwksResponse is the response format for the JWK endpoint.
type jwksResponse struct {
	// Keys is a list of public keys in JWK format.
	Keys []jwk `json:"keys"`
}

// claims represents public and private claims for a JWT token.
type claims struct {
	// Claims represents public claim values (as specified in RFC 7519).
	jwt.Claims

	// Username returns the Teleport identity of the user.
	Username string `json:"username"`

	// Roles returns the list of roles assigned to the user within Teleport.
	Roles []string `json:"roles"`

	// Traits returns the mapping of traits assigned to the user within Teleport.
	Traits map[string][]string `json:"traits"`
}

// getPublicKey fetches the public key from the JWK endpoint.
func getPublicKey(url string, insecureSkipVerify bool) (crypto.PublicKey, error) {
	// Fetch JWKs.
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecureSkipVerify,
			},
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Parse JWKs response.
	var response jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	if len(response.Keys) == 0 {
		return nil, fmt.Errorf("no keys found")
	}

	// Construct a crypto.PublicKey from the response.
	jwk := response.Keys[0]
	switch jwk.KeyType {
	case "RSA":
		return unmarshalRSAJWK(jwk)
	case "EC":
		return unmarshalECDSAJWK(jwk)
	default:
		return nil, fmt.Errorf("unsupported key type %v", jwk.KeyType)
	}
}

func unmarshalRSAJWK(jwk jwk) (*rsa.PublicKey, error) {
	if jwk.Algorithm != "RS256" {
		return nil, fmt.Errorf("unsupported algorithm %v", jwk.Algorithm)
	}

	n, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, err
	}
	e, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, err
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(n),
		E: int(new(big.Int).SetBytes(e).Uint64()),
	}, nil
}

func unmarshalECDSAJWK(jwk jwk) (*ecdsa.PublicKey, error) {
	if jwk.Algorithm != "ES256" {
		return nil, fmt.Errorf("unsupported algorithm %v", jwk.Algorithm)
	}
	if jwk.Curve != elliptic.P256().Params().Name {
		return nil, fmt.Errorf("unsupported curve %v", jwk.Curve)
	}

	x, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, err
	}
	y, err := base64.RawURLEncoding.DecodeString(jwk.Y)
	if err != nil {
		return nil, err
	}

	return &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(x),
		Y:     new(big.Int).SetBytes(y),
	}, nil
}

// verify will verify the JWT.
func verify(publicKey crypto.PublicKey, token string) (*claims, error) {
	// Parse the raw token.
	t, err := jwt.ParseSigned(token)
	if err != nil {
		return nil, err
	}

	// Validate the signature on the JWT token.
	var out claims
	if err := t.Claims(publicKey, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// validate validates the passed in claims against received claims.
func validate(claims *claims, issuer string, subject string, audience string) error {
	// Validate the claims on the JWT.
	expectedClaims := jwt.Expected{
		Issuer:   issuer,
		Subject:  subject,
		Audience: jwt.Audience{audience},
		Time:     time.Now(),
	}

	return claims.Validate(expectedClaims)
}

func printClaims(claims *claims) {
	fmt.Printf("JWT Claims\n")
	fmt.Printf("-----------\n")
	fmt.Printf("Username: %v.\n", claims.Username)
	fmt.Printf("Roles:    %v.\n", strings.Join(claims.Roles, ","))

	// Calculate the spacing between trait name and trait values.
	maxLength := 0
	printTraits := false
	for name := range claims.Traits {
		printTraits = true
		nameLength := len(name)
		if nameLength > maxLength {
			maxLength = nameLength
		}
	}
	maxLength++ // Increment by one for aligned trait values.

	// Pretty print the traits (if there are any)
	if printTraits {
		fmt.Println("Traits:")
		for name, values := range claims.Traits {
			fmt.Printf("  %-*s %s\n", maxLength, name+":", strings.Join(values, ","))
		}
	}

	fmt.Printf("Issuer:   %v.\n", claims.Issuer)
	fmt.Printf("Subject:  %v.\n", claims.Subject)
	fmt.Printf("Audience: %v.\n", claims.Audience)
}

func main() {
	// Parse flags.
	jwks := flag.String("jwks-url", "https://localhost:3080/.well-known/jwks.json", "JWK URL.")
	skipVerify := flag.Bool("insecure-skip-verify", false, "Skip server certificate validation.")
	jwt := flag.String("jwt", "", "JWT token to verify.")
	validateClaims := flag.Bool("validate-claims", true, "Validate the claims received match expected.")
	issuer := flag.String("issuer", "", "Issuer is name of the Teleport cluster.")
	subject := flag.String("subject", "", "Subject is the identity of the Teleport user.")
	audience := flag.String("audience", "", "Audience is the URI of the application.")
	flag.Parse()

	// Validate all required flags are set.
	if *jwt == "" {
		log.Fatal("JWT missing, required for validation.")
	}
	if *validateClaims {
		if *issuer == "" || *subject == "" || *audience == "" {
			log.Fatal("Issuer, Subject, and Audience required for validation.")
		}
	}

	// Fetch and construct the public key that will be used to verify the JWT.
	publicKey, err := getPublicKey(*jwks, *skipVerify)
	if err != nil {
		log.Fatalf("Failed to fetch JWKs needed to verify JWT: %v.", err)
	}

	// Verify the signature on the JWT.
	claims, err := verify(publicKey, *jwt)
	if err != nil {
		log.Fatalf("JWT signature verification failed: %v.", err)
	}

	// Validate the claims if requested.
	if *validateClaims {
		if err := validate(claims, *issuer, *subject, *audience); err != nil {
			log.Printf("Claim validation failed: %v.", err)
			printClaims(claims)
			os.Exit(1)
		}
	}

	// Print claims and exit.
	printClaims(claims)
}
