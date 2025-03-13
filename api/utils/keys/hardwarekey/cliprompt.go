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
	"fmt"
	"os"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/prompt"
)

var (
	// defaultPIN for the PIV applet. The PIN is used to change the Management Key,
	// and slots can optionally require it to perform signing operations.
	defaultPIN = "123456"
	// defaultPUK for the PIV applet. The PUK is only used to reset the PIN when
	// the card's PIN retries have been exhausted.
	defaultPUK = "12345678"
)

type CLIPrompt struct{}

func (c *CLIPrompt) AskPIN(ctx context.Context, requirement PINPromptRequirement, _ PrivateKeyInfo) (string, error) {
	message := "Enter your YubiKey PIV PIN"
	if requirement == PINOptional {
		message = "Enter your YubiKey PIV PIN [blank to use default PIN]"
	}
	password, err := prompt.Password(ctx, os.Stderr, prompt.Stdin(), message)
	return password, trace.Wrap(err)
}

func (c *CLIPrompt) Touch(_ context.Context, _ PrivateKeyInfo) error {
	_, err := fmt.Fprintln(os.Stderr, "Tap your YubiKey")
	return trace.Wrap(err)
}

func (c *CLIPrompt) ChangePIN(ctx context.Context, _ PrivateKeyInfo) (*PINAndPUK, error) {
	var pinAndPUK = &PINAndPUK{}
	for {
		fmt.Fprintf(os.Stderr, "Please set a new 6-8 character PIN.\n")
		newPIN, err := prompt.Password(ctx, os.Stderr, prompt.Stdin(), "Enter your new YubiKey PIV PIN")
		if err != nil {
			return nil, trace.Wrap(err)
		}
		newPINConfirm, err := prompt.Password(ctx, os.Stderr, prompt.Stdin(), "Confirm your new YubiKey PIV PIN")
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if newPIN != newPINConfirm {
			fmt.Fprintf(os.Stderr, "PINs do not match.\n")
			continue
		}

		if newPIN == defaultPIN {
			fmt.Fprintf(os.Stderr, "The default PIN %q is not supported.\n", defaultPIN)
			continue
		}

		if !IsPINLengthValid(newPIN) {
			fmt.Fprintf(os.Stderr, "PIN must be 6-8 characters long.\n")
			continue
		}

		pinAndPUK.PIN = newPIN
		break
	}

	puk, err := prompt.Password(ctx, os.Stderr, prompt.Stdin(), "Enter your YubiKey PIV PUK to reset PIN [blank to use default PUK]")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pinAndPUK.PUK = puk

	switch puk {
	case defaultPUK:
		fmt.Fprintf(os.Stderr, "The default PUK %q is not supported.\n", defaultPUK)
		fallthrough
	case "":
		for {
			fmt.Fprintf(os.Stderr, "Please set a new 6-8 character PUK (used to reset PIN).\n")
			newPUK, err := prompt.Password(ctx, os.Stderr, prompt.Stdin(), "Enter your new YubiKey PIV PUK")
			if err != nil {
				return nil, trace.Wrap(err)
			}
			newPUKConfirm, err := prompt.Password(ctx, os.Stderr, prompt.Stdin(), "Confirm your new YubiKey PIV PUK")
			if err != nil {
				return nil, trace.Wrap(err)
			}

			if newPUK != newPUKConfirm {
				fmt.Fprintf(os.Stderr, "PUKs do not match.\n")
				continue
			}

			if newPUK == defaultPUK {
				fmt.Fprintf(os.Stderr, "The default PUK %q is not supported.\n", defaultPUK)
				continue
			}

			if !IsPINLengthValid(newPUK) {
				fmt.Fprintf(os.Stderr, "PUK must be 6-8 characters long.\n")
				continue
			}

			pinAndPUK.PUK = newPUK
			pinAndPUK.PUKChanged = true
			break
		}
	}
	return pinAndPUK, nil
}

func (c *CLIPrompt) ConfirmSlotOverwrite(ctx context.Context, message string, _ PrivateKeyInfo) (bool, error) {
	confirmation, err := prompt.Confirmation(ctx, os.Stderr, prompt.Stdin(), message)
	return confirmation, trace.Wrap(err)
}
