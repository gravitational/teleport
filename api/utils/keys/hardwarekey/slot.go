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
type PIVSlotKey uint

const (
	pivSlotKeyBasic       PIVSlotKey = 0x9a
	pivSlotKeyTouch       PIVSlotKey = 0x9c
	pivSlotKeyTouchAndPIN PIVSlotKey = 0x9d
	pivSlotKeyPIN         PIVSlotKey = 0x9e
)

// GetDefaultSlotKey gets the default PIV slot key for the given [policy].
func GetDefaultSlotKey(policy PromptPolicy) (PIVSlotKey, error) {
	switch policy {
	case PromptPolicyNone:
		return pivSlotKeyBasic, nil
	case PromptPolicyTouch:
		return pivSlotKeyTouch, nil
	case PromptPolicyPIN:
		return pivSlotKeyPIN, nil
	case PromptPolicyTouchAndPIN:
		return pivSlotKeyTouchAndPIN, nil
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
	slotKey, err := strconv.ParseUint(string(s), 16, 32)
	if err != nil {
		return 0, trace.Wrap(err, "failed to parse %q as a uint", s)
	}

	switch p := PIVSlotKey(slotKey); p {
	case pivSlotKeyBasic, pivSlotKeyTouch, pivSlotKeyTouchAndPIN, pivSlotKeyPIN:
		return p, nil
	default:
		return 0, trace.BadParameter("invalid PIV slot %q", s)
	}
}
