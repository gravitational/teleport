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
)

// IsRequiredPolicyMet returns whether the required key policy is met by the key's policy.
func IsRequiredPolicyMet(requiredPolicy, keyPolicy PrivateKeyPolicy) bool {
	switch requiredPolicy {
	case PrivateKeyPolicyNone:
		return true
	case PrivateKeyPolicyHardwareKey:
		return keyPolicy.IsHardwareKeyPolicy()
	case PrivateKeyPolicyHardwareKeyTouch:
		return keyPolicy.isHardwareKeyTouchVerified()
	case PrivateKeyPolicyHardwareKeyPIN:
		return keyPolicy.isHardwareKeyPINVerified()
	case PrivateKeyPolicyHardwareKeyTouchAndPIN:
		return keyPolicy.isHardwareKeyTouchVerified() && keyPolicy.isHardwareKeyPINVerified()
	}

	return false
}

// Deprecated in favor of IsRequiredPolicyMet.
// TODO(Joerger): delete once reference in /e is replaced.
func (requiredPolicy PrivateKeyPolicy) VerifyPolicy(keyPolicy PrivateKeyPolicy) error {
	if !IsRequiredPolicyMet(requiredPolicy, keyPolicy) {
		return NewPrivateKeyPolicyError(requiredPolicy)
	}
	return nil
}

// IsHardwareKeyPolicy return true if this private key policy requires a hardware key.
func (p PrivateKeyPolicy) IsHardwareKeyPolicy() bool {
	switch p {
	case PrivateKeyPolicyHardwareKey,
		PrivateKeyPolicyHardwareKeyTouch,
		PrivateKeyPolicyHardwareKeyPIN,
		PrivateKeyPolicyHardwareKeyTouchAndPIN:
		return true
	}
	return false
}

// MFAVerified checks that private keys with this key policy count as MFA verified.
// Both Hardware key touch and pin are count as MFA verification.
func (p PrivateKeyPolicy) MFAVerified() bool {
	return p.isHardwareKeyTouchVerified() || p.isHardwareKeyPINVerified()
}

func (p PrivateKeyPolicy) isHardwareKeyTouchVerified() bool {
	switch p {
	case PrivateKeyPolicyHardwareKeyTouch, PrivateKeyPolicyHardwareKeyTouchAndPIN:
		return true
	}
	return false
}

func (p PrivateKeyPolicy) isHardwareKeyPINVerified() bool {
	switch p {
	case PrivateKeyPolicyHardwareKeyPIN, PrivateKeyPolicyHardwareKeyTouchAndPIN:
		return true
	}
	return false
}

func (p PrivateKeyPolicy) validate() error {
	switch p {
	case PrivateKeyPolicyNone,
		PrivateKeyPolicyHardwareKey,
		PrivateKeyPolicyHardwareKeyTouch,
		PrivateKeyPolicyHardwareKeyPIN,
		PrivateKeyPolicyHardwareKeyTouchAndPIN:
		return nil
	}
	return trace.BadParameter("%q is not a valid key policy", p)
}

// GetPolicyFromSet returns least restrictive policy necessary to meet the given set of policies.
func GetPolicyFromSet(policies []PrivateKeyPolicy) (PrivateKeyPolicy, error) {
	setPolicy := PrivateKeyPolicyNone
	for _, policy := range policies {
		if !IsRequiredPolicyMet(policy, setPolicy) {
			if IsRequiredPolicyMet(setPolicy, policy) {
				// Upgrade set policy to stricter policy.
				setPolicy = policy
			} else if IsRequiredPolicyMet(policy, PrivateKeyPolicyHardwareKeyTouchAndPIN) &&
				IsRequiredPolicyMet(setPolicy, PrivateKeyPolicyHardwareKeyTouchAndPIN) {
				// neither policy is met by the other (pin or touch), but both are met by stricter pin+touch policy).
				setPolicy = PrivateKeyPolicyHardwareKeyTouchAndPIN
			} else {
				return PrivateKeyPolicyNone, trace.BadParameter("private key policy requirements %q and %q are incompatible, please contact the cluster administrator", policy, setPolicy)
			}
		}
	}

	return setPolicy, nil
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
