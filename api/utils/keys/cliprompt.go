//go:build piv && !pivtest

// Copyright 2024 Gravitational, Inc.
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

package keys

import (
	"context"
	"fmt"
	"os"

	"github.com/go-piv/piv-go/piv"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/prompt"
)

type cliPrompt struct{}

func (c *cliPrompt) AskPIN(ctx context.Context, requirement PINPromptRequirement) (string, error) {
	message := "Enter your YubiKey PIV PIN"
	if requirement == PINOptional {
		message = "Enter your YubiKey PIV PIN [blank to use default PIN]"
	}
	password, err := prompt.Password(ctx, os.Stderr, prompt.Stdin(), message)
	return password, trace.Wrap(err)
}

func (c *cliPrompt) Touch(_ context.Context) error {
	_, err := fmt.Fprintln(os.Stderr, "Tap your YubiKey")
	return trace.Wrap(err)
}

func (c *cliPrompt) ChangePIN(ctx context.Context) (*PINAndPUK, error) {
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

		if newPIN == piv.DefaultPIN {
			fmt.Fprintf(os.Stderr, "The default PIN %q is not supported.\n", piv.DefaultPIN)
			continue
		}

		if !isPINLengthValid(newPIN) {
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
	case piv.DefaultPUK:
		fmt.Fprintf(os.Stderr, "The default PUK %q is not supported.\n", piv.DefaultPUK)
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

			if newPUK == piv.DefaultPUK {
				fmt.Fprintf(os.Stderr, "The default PUK %q is not supported.\n", piv.DefaultPUK)
				continue
			}

			if !isPINLengthValid(newPUK) {
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

func (c *cliPrompt) ConfirmSlotOverwrite(ctx context.Context, message string) (bool, error) {
	confirmation, err := prompt.Confirmation(ctx, os.Stderr, prompt.Stdin(), message)
	return confirmation, trace.Wrap(err)
}

func isPINLengthValid(pin string) bool {
	return len(pin) >= 6 && len(pin) <= 8
}
