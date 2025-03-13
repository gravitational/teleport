//go:build piv && !pivtest

/*
Copyright 2025 Gravitational, Inc.
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
	"github.com/go-piv/piv-go/piv"
)

// GetPrivateKeyPolicyFromAttestation returns the PrivateKeyPolicy satisfied by the given hardware key attestation.
// TODO(Joerger): Move to /e where this is used.
func GetPrivateKeyPolicyFromAttestation(att *piv.Attestation) PrivateKeyPolicy {
	if att == nil {
		return PrivateKeyPolicyNone
	}

	isTouchPolicy := att.TouchPolicy == piv.TouchPolicyCached ||
		att.TouchPolicy == piv.TouchPolicyAlways

	isPINPolicy := att.PINPolicy == piv.PINPolicyOnce ||
		att.PINPolicy == piv.PINPolicyAlways

	switch {
	case isPINPolicy && isTouchPolicy:
		return PrivateKeyPolicyHardwareKeyTouchAndPIN
	case isPINPolicy:
		return PrivateKeyPolicyHardwareKeyPIN
	case isTouchPolicy:
		return PrivateKeyPolicyHardwareKeyTouch
	default:
		return PrivateKeyPolicyHardwareKey
	}
}
