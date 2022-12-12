/*
Copyright 2022 Gravitational, Inc.
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

package keys

import (
	"fmt"
	"regexp"

	"github.com/gravitational/trace"
)

// PrivateKeyPolicy is a requirement for client private key storage.
type PrivateKeyPolicy string

const (
	// PrivateKeyPolicyNone means that the client can store their private keys
	// anywhere (usually on disk).
	PrivateKeyPolicyNone PrivateKeyPolicy = "none"
	// PrivateKeyPolicyHardwareKey means that the client must use a valid
	// hardware key to generate and store their private keys securely.
	PrivateKeyPolicyHardwareKey PrivateKeyPolicy = "hardware_key"
	// PrivateKeyPolicyHardwareKeyTouch means that the client must use a valid
	// hardware key to generate and store their private keys securely, and
	// this key must require touch to be accessed and used.
	PrivateKeyPolicyHardwareKeyTouch PrivateKeyPolicy = "hardware_key_touch"
)

// VerifyPolicy verifies that the given policy meets the requirements of this policy.
// If not, it will return a private key policy error, which can be parsed to retrieve
// the unmet policy.
func (p PrivateKeyPolicy) VerifyPolicy(policy PrivateKeyPolicy) error {
	switch p {
	case PrivateKeyPolicyNone:
		return nil
	case PrivateKeyPolicyHardwareKey:
		if policy == PrivateKeyPolicyHardwareKey || policy == PrivateKeyPolicyHardwareKeyTouch {
			return nil
		}
	case PrivateKeyPolicyHardwareKeyTouch:
		if policy == PrivateKeyPolicyHardwareKeyTouch {
			return nil
		}
	}
	return NewPrivateKeyPolicyError(p)
}

func (p PrivateKeyPolicy) validate() error {
	switch p {
	case PrivateKeyPolicyNone, PrivateKeyPolicyHardwareKey, PrivateKeyPolicyHardwareKeyTouch:
		return nil
	}
	return trace.BadParameter("%q is not a valid key policy", p)
}

var privateKeyPolicyErrRegex = regexp.MustCompile(`private key policy not met: (\w+)`)

func NewPrivateKeyPolicyError(p PrivateKeyPolicy) error {
	return trace.BadParameter(fmt.Sprintf("private key policy not met: %s", p))
}

// ParsePrivateKeyPolicyError checks if the given error is a private key policy
// error and returns the contained unmet PrivateKeyPolicy.
func ParsePrivateKeyPolicyError(err error) (PrivateKeyPolicy, error) {
	// subMatches should have two groups - the full string and the policy "(\w+)"
	subMatches := privateKeyPolicyErrRegex.FindStringSubmatch(err.Error())
	if subMatches == nil || len(subMatches) != 2 {
		return "", trace.BadParameter("provided error is not a key policy error")
	}

	policy := PrivateKeyPolicy(subMatches[1])
	if err := policy.validate(); err != nil {
		return "", trace.Wrap(err)
	}
	return policy, nil
}

// IsPrivateKeyPolicyError returns true if the given error is a private key policy error.
func IsPrivateKeyPolicyError(err error) bool {
	return privateKeyPolicyErrRegex.MatchString(err.Error())
}
