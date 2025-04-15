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

// Package hardwarekey defines types and interfaces for hardware private keys.
package hardwarekey

import (
	"strconv"

	"github.com/gravitational/trace"
)

// PIVSlotKey is the key reference for a specific PIV slot.
//
// See: https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf#page=32
type PIVSlotKey uint32

const (
	PIVSlot9A PIVSlotKey = 0x9a
	PIVSlot9C PIVSlotKey = 0x9c
	PIVSlot9D PIVSlotKey = 0x9d
	PIVSlot9E PIVSlotKey = 0x9e
)

// Validate the slot key value.
func (k PIVSlotKey) Validate() error {
	switch k {
	case PIVSlot9A, PIVSlot9C, PIVSlot9D, PIVSlot9E:
		return nil
	default:
		return trace.BadParameter("invalid PIV slot key %d", k)
	}
}

// GetDefaultSlotKey gets the default PIV slot key for the given [policy].
//   - 9A for PromptPolicyNone
//   - 9C for PromptPolicyTouch
//   - 9D for PromptPolicyTouchAndPIN
//   - 9E for PromptPolicyPIN
func GetDefaultSlotKey(policy PromptPolicy) (PIVSlotKey, error) {
	switch policy {
	case PromptPolicyNone:
		return PIVSlot9A, nil
	case PromptPolicyTouch:
		return PIVSlot9C, nil
	case PromptPolicyPIN:
		return PIVSlot9E, nil
	case PromptPolicyTouchAndPIN:
		return PIVSlot9D, nil
	default:
		return 0, trace.BadParameter("unexpected prompt policy %v", policy)
	}
}

// PIVSlotKeyString is the string representation of a [PIVSlotKey].
type PIVSlotKeyString string

// Validate that [s] parses into a valid [PIVSlotKey].
func (s PIVSlotKeyString) Validate() error {
	_, err := s.Parse()
	return trace.Wrap(err)
}

// Parse [s] into a [PIVSlotKey].
func (s PIVSlotKeyString) Parse() (PIVSlotKey, error) {
	slotKeyUint, err := strconv.ParseUint(string(s), 16, 32)
	if err != nil {
		return 0, trace.Wrap(err, "failed to parse %q as a uint", s)
	}

	slotKey := PIVSlotKey(slotKeyUint)
	if err := slotKey.Validate(); err != nil {
		return 0, trace.Wrap(err)
	}

	return slotKey, nil
}
