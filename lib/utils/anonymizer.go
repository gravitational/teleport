/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"sync"

	"github.com/gravitational/trace"
)

// Anonymizer defines an interface for anonymizing data
type Anonymizer interface {
	// Anonymize returns anonymized string from the provided data
	Anonymize(data []byte) string

	// AnonymizeString anonymizes the given string data using HMAC
	AnonymizeString(s string) string

	// AnonymizeNonEmpty anonymizes the given string into bytes if the string is
	// nonempty, otherwise returns an empty slice.
	AnonymizeNonEmpty(s string) []byte

	// SetAnonymizationKey updates the underlying anonymization key.
	SetAnonymizationKey(k []byte)
}

// HMACAnonymizer implements anonymization using HMAC
type HMACAnonymizer struct {
	// key is the HMAC key
	key []byte
	mu  sync.RWMutex
}

var _ Anonymizer = (*HMACAnonymizer)(nil)

func (a *HMACAnonymizer) SetAnonymizationKey(k []byte) {
	a.mu.Lock()
	a.key = k
	a.mu.Unlock()
}

// NewHMACAnonymizer returns a new HMAC-based anonymizer
func NewHMACAnonymizer(key string) (*HMACAnonymizer, error) {
	if strings.TrimSpace(key) == "" {
		return nil, trace.BadParameter("HMAC key must not be empty")
	}
	return &HMACAnonymizer{key: []byte(key)}, nil
}

// Anonymize anonymizes the provided data using HMAC
func (a *HMACAnonymizer) Anonymize(data []byte) string {
	a.mu.RLock()
	k := a.key
	a.mu.RUnlock()

	h := hmac.New(sha256.New, k)
	h.Write(data)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// AnonymizeString anonymizes the given string data using HMAC
func (a *HMACAnonymizer) AnonymizeString(s string) string {
	return a.Anonymize([]byte(s))
}

// AnonymizeNonEmpty implements [Anonymizer].
func (a *HMACAnonymizer) AnonymizeNonEmpty(s string) []byte {
	if s == "" {
		return nil
	}

	a.mu.RLock()
	k := a.key
	a.mu.RUnlock()

	h := hmac.New(sha256.New, k)
	h.Write([]byte(s))
	return h.Sum(nil)
}
