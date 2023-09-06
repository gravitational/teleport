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
	// PrivateKeyPolicyHardwareKeyPIN means that the client must use a valid
	// hardware key to generate and store their private keys securely, and
	// this key must require pin to be accessed and used.
	PrivateKeyPolicyHardwareKeyPIN PrivateKeyPolicy = "hardware_key_pin"
	// PrivateKeyPolicyHardwareKeyTouchAndPIN means that the client must use a valid
	// hardware key to generate and store their private keys securely, and
	// this key must require touch and pin to be accessed and used.
	PrivateKeyPolicyHardwareKeyTouchAndPIN PrivateKeyPolicy = "hardware_key_touch_and_pin"
	// PrivateKeyPolicyWebSession is a special case used for Web Sessions. This policy
	// implies that the client private key and certificate are stored in the Proxy
	// Process Memory and Auth Storage. These certs do not leave the Proxy/Auth
	// services, but the Web Client receives a Web Cookie which can be used to
	// make requests with the server-side client key+cert.
	//
	// This policy does not provide the same hardware key guarantee as the above policies.
	// Instead, this policy must be accompanied by WebAuthn prompts for important operations
	// in order to pass hardware key policy requirements.
	PrivateKeyPolicyWebSession PrivateKeyPolicy = "web_session"
)

// VerifyPolicy verifies that the given policy meets the requirements of this policy.
// If not, it will return a private key policy error, which can be parsed to retrieve
// the unmet policy.
func (p PrivateKeyPolicy) VerifyPolicy(policy PrivateKeyPolicy) error {
	if err := policy.validate(); err != nil {
		return trace.Wrap(err)
	}

	if policy == PrivateKeyPolicyWebSession {
		return nil
	}

	switch p {
	case PrivateKeyPolicyNone:
		return nil
	case PrivateKeyPolicyHardwareKey:
		if policy != PrivateKeyPolicyNone {
			return nil
		}
	case PrivateKeyPolicyHardwareKeyTouch:
		if policy == PrivateKeyPolicyHardwareKeyTouch || policy == PrivateKeyPolicyHardwareKeyTouchAndPIN {
			return nil
		}
	case PrivateKeyPolicyHardwareKeyPIN:
		if policy == PrivateKeyPolicyHardwareKeyPIN || policy == PrivateKeyPolicyHardwareKeyTouchAndPIN {
			return nil
		}
	case PrivateKeyPolicyHardwareKeyTouchAndPIN:
		if policy == PrivateKeyPolicyHardwareKeyTouchAndPIN {
			return nil
		}
	}

	return NewPrivateKeyPolicyError(p)
}

// MFAVerified checks whether the given private key policy counts towards MFA verification.
func (p PrivateKeyPolicy) MFAVerified() bool {
	return p == PrivateKeyPolicyHardwareKeyTouch || p == PrivateKeyPolicyHardwareKeyTouchAndPIN
}

func (p PrivateKeyPolicy) validate() error {
	switch p {
	case PrivateKeyPolicyNone,
		PrivateKeyPolicyHardwareKey,
		PrivateKeyPolicyHardwareKeyTouch,
		PrivateKeyPolicyHardwareKeyPIN,
		PrivateKeyPolicyHardwareKeyTouchAndPIN,
		PrivateKeyPolicyWebSession:
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
