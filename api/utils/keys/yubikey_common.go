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
	"context"

	"github.com/gravitational/trace"
)

// HardwareKeyPrompt provides methods to interact with a YubiKey hardware key.
type HardwareKeyPrompt interface {
	// AskPIN prompts the user for a PIN.
	// The requirement tells if the PIN is required or optional.
	AskPIN(ctx context.Context, requirement PINPromptRequirement, keyInfo KeyInfo) (string, error)
	// Touch prompts the user to touch the hardware key.
	Touch(ctx context.Context, keyInfo KeyInfo) error
	// ChangePIN asks for a new PIN.
	// If the PUK has a default value, it should ask for the new value for it.
	// It is up to the implementer how the validation is handled.
	// For example, CLI prompt can ask for a valid PIN/PUK in a loop, a GUI
	// prompt can use the frontend validation.
	ChangePIN(ctx context.Context, keyInfo KeyInfo) (*PINAndPUK, error)
	// ConfirmSlotOverwrite asks the user if the slot's private key and certificate can be overridden.
	ConfirmSlotOverwrite(ctx context.Context, message string, keyInfo KeyInfo) (bool, error)
}

// PINPromptRequirement specifies whether a PIN is required.
type PINPromptRequirement int

const (
	// PINOptional allows the user to proceed without entering a PIN.
	PINOptional PINPromptRequirement = iota
	// PINRequired enforces that a PIN must be entered to proceed.
	PINRequired
)

// PINAndPUK describes a response returned from HardwareKeyPrompt.ChangePIN.
type PINAndPUK struct {
	// New PIN set by the user.
	PIN string
	// PUK used to change the PIN.
	// This is a new PUK if it has not been changed (from the default PUK).
	PUK string
	// PUKChanged is true if the user changed the default PUK.
	PUKChanged bool
}

// GetYubiKeyPrivateKey attempt to retrieve a YubiKey private key matching the given hardware key policy
// from the given slot. If slot is unspecified, the default slot for the given key policy will be used.
// If the slot is empty, a new private key matching the given policy will be generated in the slot.
//   - hardware_key: 9a
//   - hardware_key_touch: 9c
//   - hardware_key_pin: 9d
//   - hardware_key_touch_pin: 9e
func GetYubiKeyPrivateKey(ctx context.Context, policy PrivateKeyPolicy, slot PIVSlot, customPrompt HardwareKeyPrompt) (*PrivateKey, error) {
	priv, err := getOrGenerateYubiKeyPrivateKey(ctx, policy, slot, customPrompt)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get a YubiKey private key")
	}
	return priv, nil
}

// PIVSlot is the string representation of a PIV slot. e.g. "9a".
type PIVSlot string

// Validate that the PIV slot is a valid value.
func (s PIVSlot) Validate() error {
	return trace.Wrap(s.validate())
}
