// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hardwarekey

import (
	"context"

	"github.com/gravitational/trace"
)

var (
	// PromptPolicyNone does not require touch or pin.
	PromptPolicyNone = PromptPolicy{TouchRequired: false, PINRequired: false}
	// PromptPolicyTouch requires touch.
	PromptPolicyTouch = PromptPolicy{TouchRequired: true, PINRequired: false}
	// PromptPolicyPIN requires pin.
	PromptPolicyPIN = PromptPolicy{TouchRequired: false, PINRequired: true}
	// PromptPolicyTouchAndPIN requires touch and pin.
	PromptPolicyTouchAndPIN = PromptPolicy{TouchRequired: true, PINRequired: true}
)

// PromptPolicy specifies a hardware private key's PIN/touch prompt policies.
type PromptPolicy struct {
	// TouchRequired means that touch is required for signatures.
	TouchRequired bool
	// PINRequired means that PIN is required for signatures.
	PINRequired bool
}

// Prompt provides methods to interact with a hardware [PrivateKey].
type Prompt interface {
	// AskPIN prompts the user for a PIN.
	// The requirement tells if the PIN is required or optional.
	AskPIN(ctx context.Context, requirement PINPromptRequirement) (string, error)
	// Touch prompts the user to touch the hardware key.
	Touch(ctx context.Context) error
	// ChangePIN asks for a new PIN.
	// If the PUK has a default value, it should ask for the new value for it.
	// It is up to the implementer how the validation is handled.
	// For example, CLI prompt can ask for a valid PIN/PUK in a loop, a GUI
	// prompt can use the frontend validation.
	ChangePIN(ctx context.Context) (*PINAndPUK, error)
	// ConfirmSlotOverwrite asks the user if the slot's private key and certificate can be overridden.
	ConfirmSlotOverwrite(ctx context.Context, message string) (bool, error)
}

// PINPromptRequirement specifies whether a PIN is required.
type PINPromptRequirement int

const (
	// PINOptional allows the user to proceed without entering a PIN.
	PINOptional PINPromptRequirement = iota
	// PINRequired enforces that a PIN must be entered to proceed.
	PINRequired
)

// PINAndPUK describes a response returned from [Prompt].ChangePIN.
type PINAndPUK struct {
	// New PIN set by the user.
	PIN string
	// PUK used to change the PIN.
	// This is a new PUK if it has not been changed (from the default PUK).
	PUK string
	// PUKChanged is true if the user changed the default PUK.
	PUKChanged bool
}

// Validate the user-provided PIN and PUK.
func (p PINAndPUK) Validate() error {
	if !isPINLengthValid(p.PIN) {
		return trace.BadParameter("PIN must be 6-8 characters long")
	}
	if p.PIN == DefaultPIN {
		return trace.BadParameter("The default PIN is not supported")
	}
	if !isPINLengthValid(p.PUK) {
		return trace.BadParameter("PUK must be 6-8 characters long")
	}
	if p.PUK == DefaultPUK {
		return trace.BadParameter("The default PUK is not supported")
	}
	return nil
}

// isPINLengthValid returns whether the given PIV PIN, or PUK, is of valid length (6-8 characters).
func isPINLengthValid(pin string) bool {
	return len(pin) >= 6 && len(pin) <= 8
}
