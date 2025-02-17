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

// IsSatisfiedBy returns whether this key policy is satisfied by the given key policy.
func (requiredPolicy PrivateKeyPolicy) IsSatisfiedBy(keyPolicy PrivateKeyPolicy) bool {
	// Web sessions are treated as a special case that meets all private key policy requirements.
	if keyPolicy == PrivateKeyPolicyWebSession {
		return true
	}

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
//
// Note: MFA checks with private key policies are only performed during the establishment
// of the connection, during the TLS/SSH handshake. For long term connections, MFA should
// be re-verified through other methods (e.g. webauthn).
func (p PrivateKeyPolicy) MFAVerified() bool {
	return p.isHardwareKeyTouchVerified() || p.isHardwareKeyPINVerified()
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

// PolicyThatSatisfiesSet returns least restrictive policy necessary to satisfy the given set of policies.
func PolicyThatSatisfiesSet(policies []PrivateKeyPolicy) (PrivateKeyPolicy, error) {
	setPolicy := PrivateKeyPolicyNone
	for _, policy := range policies {
		if policy.IsSatisfiedBy(setPolicy) {
			continue
		}

		switch {
		case setPolicy.IsSatisfiedBy(policy):
			// Upgrade set policy to stricter policy.
			setPolicy = policy

		case policy.IsSatisfiedBy(PrivateKeyPolicyHardwareKeyTouchAndPIN) &&
			setPolicy.IsSatisfiedBy(PrivateKeyPolicyHardwareKeyTouchAndPIN):
			// Neither policy is met by the other (pin or touch), but both are met by
			// stricter pin+touch policy.
			setPolicy = PrivateKeyPolicyHardwareKeyTouchAndPIN

		default:
			// Currently, "hardware_key_touch_and_pin" is the strictest policy available and
			// meets every other required policy. However, in the future we may add policy
			// requirements that are mutually exclusive, so this logic is future proofed.
			return PrivateKeyPolicyNone, trace.BadParameter(""+
				"private key policy requirements %q and %q are incompatible, "+
				"please contact the cluster administrator", policy, setPolicy)
		}
	}

	return setPolicy, nil
}

var privateKeyPolicyErrRegex = regexp.MustCompile(`private key policy not (met|satisfied): (\w+)`)

func NewPrivateKeyPolicyError(p PrivateKeyPolicy) error {
	// TODO(Joerger): Replace with "private key policy not satisfied" in 16.0.0
	return trace.BadParameter("private key policy not met: %s", p)
}

// ParsePrivateKeyPolicyError checks if the given error is a private key policy
// error and returns the contained unsatisfied PrivateKeyPolicy.
func ParsePrivateKeyPolicyError(err error) (PrivateKeyPolicy, error) {
	// subMatches should have two groups - the full string and the policy "(\w+)"
	subMatches := privateKeyPolicyErrRegex.FindStringSubmatch(err.Error())
	if subMatches == nil || len(subMatches) != 3 {
		return "", trace.BadParameter("provided error is not a key policy error")
	}

	policy := PrivateKeyPolicy(subMatches[2])
	if err := policy.validate(); err != nil {
		return "", trace.Wrap(err)
	}
	return policy, nil
}

// IsPrivateKeyPolicyError returns true if the given error is a private key policy error.
func IsPrivateKeyPolicyError(err error) bool {
	if err == nil {
		return false
	}
	return privateKeyPolicyErrRegex.MatchString(err.Error())
}
