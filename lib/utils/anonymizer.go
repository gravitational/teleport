/*
Copyright 2018 Gravitational, Inc.

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

package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"

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
}

// hmacAnonymizer implements anonymization using HMAC
type HMACAnonymizer struct {
	// key is the HMAC key
	key []byte
}

var _ Anonymizer = (*HMACAnonymizer)(nil)

// NewHMACAnonymizer returns a new HMAC-based anonymizer
func NewHMACAnonymizer(key string) (*HMACAnonymizer, error) {
	if strings.TrimSpace(key) == "" {
		return nil, trace.BadParameter("HMAC key must not be empty")
	}
	return &HMACAnonymizer{key: []byte(key)}, nil
}

// Anonymize anonymizes the provided data using HMAC
func (a *HMACAnonymizer) Anonymize(data []byte) string {
	h := hmac.New(sha256.New, a.key)
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
	h := hmac.New(sha256.New, a.key)
	h.Write([]byte(s))
	return h.Sum(nil)
}
