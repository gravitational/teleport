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
}

// hmacAnonymizer implements anonymization using HMAC
type hmacAnonymizer struct {
	// key is the HMAC key
	key string
}

// NewHMACAnonymizer returns a new HMAC-based anonymizer
func NewHMACAnonymizer(key string) (*hmacAnonymizer, error) {
	if strings.TrimSpace(key) == "" {
		return nil, trace.BadParameter("HMAC key must not be empty")
	}
	return &hmacAnonymizer{
		key: key,
	}, nil
}

// Anonymize anonymizes the provided data using HMAC
func (a *hmacAnonymizer) Anonymize(data []byte) string {
	h := hmac.New(sha256.New, []byte(a.key))
	h.Write(data)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
