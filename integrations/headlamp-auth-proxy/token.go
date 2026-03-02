/*
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
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenSigner mints and validates HMAC-signed JWTs that carry user identity
// between the front proxy and the K8s proxy within the same pod.
type TokenSigner struct {
	key []byte
}

// InternalClaims are the claims encoded in the internal HMAC token.
// The token is a valid JWT so Headlamp's /me endpoint can decode it.
type InternalClaims struct {
	jwt.RegisteredClaims
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Groups   []string `json:"groups"`
}

// Mint creates a new HMAC-signed JWT encoding the user identity.
func (s *TokenSigner) Mint(username string, groups []string) (string, error) {
	now := time.Now()
	claims := InternalClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
		Username: username,
		Email:    username,
		Groups:   groups,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.key)
}

// Validate parses and validates an internal HMAC token.
func (s *TokenSigner) Validate(tokenString string) (*InternalClaims, error) {
	var claims InternalClaims
	_, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.key, nil
	})
	if err != nil {
		return nil, err
	}
	return &claims, nil
}
