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

package hardwarekey_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/api/utils/prompt"
)

func TestChangePIN(t *testing.T) {
	const validPINPUK = "1234567"

	for _, tc := range []struct {
		name            string
		inputs          []string
		expectPINAndPUK *hardwarekey.PINAndPUK
		expectOutput    string
		expectError     error
	}{
		{
			name:        "no input",
			expectError: context.DeadlineExceeded,
		},
		{
			name: "set pin short",
			inputs: []string{
				"123", // pin
				"123", // confirm
			},
			expectOutput: "PIN must be 6-8 characters long.",
			expectError:  context.DeadlineExceeded,
		}, {
			name: "set pin mismatch",
			inputs: []string{
				"",                     // pin
				hardwarekey.DefaultPIN, // confirm
			},
			expectOutput: "PINs do not match.",
			expectError:  context.DeadlineExceeded,
		}, {
			name: "set pin default",
			inputs: []string{
				hardwarekey.DefaultPIN, // pin
				hardwarekey.DefaultPIN, // confirm
			},
			expectOutput: fmt.Sprintf("The default PIN %q is not supported.", hardwarekey.DefaultPIN),
			expectError:  context.DeadlineExceeded,
		}, {
			name: "set puk short",
			inputs: []string{
				validPINPUK, // pin
				validPINPUK, // confirm
				"",          // empty puk -> trigger set puk
				"123",       // puk
				"123",       // confirm
			},
			expectOutput: "PUK must be 6-8 characters long.",
			expectError:  context.DeadlineExceeded,
		}, {
			name: "set puk mismatch",
			inputs: []string{
				validPINPUK, // pin
				validPINPUK, // confirm
				"",          // empty puk -> trigger set puk
				"",          // puk
				validPINPUK, // confirm
			},
			expectOutput: "PUKs do not match.",
			expectError:  context.DeadlineExceeded,
		}, {
			name: "set puk default",
			inputs: []string{
				validPINPUK,            // pin
				validPINPUK,            // confirm
				"",                     // empty puk -> trigger set puk
				hardwarekey.DefaultPUK, // puk
				hardwarekey.DefaultPUK, // confirm
			},
			expectOutput: fmt.Sprintf("The default PUK %q is not supported.", hardwarekey.DefaultPUK),
			expectError:  context.DeadlineExceeded,
		}, {
			name: "set puk from empty",
			inputs: []string{
				validPINPUK, // pin
				validPINPUK, // confirm
				"",          // empty puk -> trigger set puk
				validPINPUK, // puk
				validPINPUK, // confirm
			},
			expectPINAndPUK: &hardwarekey.PINAndPUK{
				PIN:        validPINPUK,
				PUK:        validPINPUK,
				PUKChanged: true,
			},
		}, {
			name: "set puk from default",
			inputs: []string{
				validPINPUK,            // pin
				validPINPUK,            // confirm
				hardwarekey.DefaultPUK, // default puk -> trigger set puk
				validPINPUK,            // puk
				validPINPUK,            // confirm
			},
			expectPINAndPUK: &hardwarekey.PINAndPUK{
				PIN:        validPINPUK,
				PUK:        validPINPUK,
				PUKChanged: true,
			},
		}, {
			name: "valid pin, valid puk",
			inputs: []string{
				validPINPUK, // pin
				validPINPUK, // confirm
				validPINPUK, // puk
			},
			expectPINAndPUK: &hardwarekey.PINAndPUK{
				PIN: validPINPUK,
				PUK: validPINPUK,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			promptWriter := bytes.NewBuffer([]byte{})
			promptReader := prompt.NewFakeReader()
			prompt := hardwarekey.NewCLIPrompt(promptWriter, promptReader)

			for _, input := range tc.inputs {
				promptReader.AddString(input)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()

			PINAndPUK, err := prompt.ChangePIN(ctx, hardwarekey.ContextualKeyInfo{})
			require.ErrorIs(t, err, tc.expectError)
			require.Equal(t, tc.expectPINAndPUK, PINAndPUK)
		})
	}
}
