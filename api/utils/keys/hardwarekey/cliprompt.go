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
	"io"
	"os"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/prompt"
)

var (
	// DefaultPIN for the PIV applet. The PIN is used to change the Management Key,
	// and slots can optionally require it to perform signing operations.
	DefaultPIN = "123456"
	// DefaultPUK for the PIV applet. The PUK is only used to reset the PIN when
	// the card's PIN retries have been exhausted.
	DefaultPUK = "12345678"
)

type cliPrompt struct {
	writer io.Writer
	reader prompt.StdinReader
}

// NewStdCLIPrompt returns a new CLIPrompt with stderr and stdout.
func NewStdCLIPrompt() *cliPrompt {
	return &cliPrompt{
		writer: os.Stderr,
		reader: prompt.Stdin(),
	}
}

// NewStdCLIPrompt returns a new CLIPrompt with the given writer and reader.
// Used in tests.
func NewCLIPrompt(w io.Writer, r prompt.StdinReader) *cliPrompt {
	return &cliPrompt{
		writer: w,
		reader: r,
	}
}

// AskPIN prompts the user for a PIN. If the requirement is [PINOptional],
// the prompt will offer the default PIN as a default value.
func (c *cliPrompt) AskPIN(ctx context.Context, requirement PINPromptRequirement, keyInfo ContextualKeyInfo) (string, error) {
	msg := "Enter your YubiKey PIV PIN"

	// The user may need to set their PIN for the first time during login,
	// give them a hint to continue to setting the PIN.
	if requirement == PINOptional {
		msg += " [blank to use default PIN]"
	}

	// If this is a hardware key agent request with command context info,
	// include the command in the prompt.
	if keyInfo.AgentKeyInfo.Command != "" {
		msg = fmt.Sprintf("%v to continue with command %q", msg, keyInfo.AgentKeyInfo.Command)
	}

	pin, err := prompt.Password(ctx, c.writer, c.reader, msg)
	if err != nil {
		return "", nil
	}

	if pin == "" {
		pin = DefaultPIN
	}

	return pin, trace.Wrap(err)
}

// Touch prompts the user to touch the hardware key.
func (c *cliPrompt) Touch(_ context.Context, keyInfo ContextualKeyInfo) error {
	msg := "Tap your YubiKey"
	if keyInfo.AgentKeyInfo.Command != "" {
		msg = fmt.Sprintf("%v to continue with command %q", msg, keyInfo.AgentKeyInfo.Command)
	}

	_, err := fmt.Fprintln(c.writer, msg)
	return trace.Wrap(err)
}

// ChangePIN asks for a new PIN and the current PUK to change to the new PIN.
// If the provided PUK is the default value, it will ask for a new PUK as well.
// If an invalid PIN or PUK is provided, the user will be re-prompted until a
// valid value is provided.
func (c *cliPrompt) ChangePIN(ctx context.Context, _ ContextualKeyInfo) (*PINAndPUK, error) {
	fmt.Fprintf(os.Stderr, "The default PIN %q is not supported.\n", DefaultPIN)

	var pinAndPUK = &PINAndPUK{}
	for {
		fmt.Fprintf(c.writer, "Please set a new 6-8 character PIN.\n")
		newPIN, err := prompt.Password(ctx, c.writer, c.reader, "Enter your new YubiKey PIV PIN")
		if err != nil {
			return nil, trace.Wrap(err)
		}
		newPINConfirm, err := prompt.Password(ctx, c.writer, c.reader, "Confirm your new YubiKey PIV PIN")
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if newPIN != newPINConfirm {
			fmt.Fprintf(c.writer, "PINs do not match.\n")
			continue
		}

		if newPIN == DefaultPIN {
			fmt.Fprintf(c.writer, "The default PIN %q is not supported.\n", DefaultPIN)
			continue
		}

		if !isPINLengthValid(newPIN) {
			fmt.Fprintf(c.writer, "PIN must be 6-8 characters long.\n")
			continue
		}

		pinAndPUK.PIN = newPIN
		break
	}

	puk, err := prompt.Password(ctx, c.writer, c.reader, "Enter your YubiKey PIV PUK to reset PIN [blank to use default PUK]")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pinAndPUK.PUK = puk

	switch puk {
	case DefaultPUK:
		fmt.Fprintf(c.writer, "The default PUK %q is not supported.\n", DefaultPUK)
		fallthrough
	case "":
		for {
			fmt.Fprintf(c.writer, "Please set a new 6-8 character PUK (used to reset PIN).\n")
			newPUK, err := prompt.Password(ctx, c.writer, c.reader, "Enter your new YubiKey PIV PUK")
			if err != nil {
				return nil, trace.Wrap(err)
			}
			newPUKConfirm, err := prompt.Password(ctx, c.writer, c.reader, "Confirm your new YubiKey PIV PUK")
			if err != nil {
				return nil, trace.Wrap(err)
			}

			if newPUK != newPUKConfirm {
				fmt.Fprintf(c.writer, "PUKs do not match.\n")
				continue
			}

			if newPUK == DefaultPUK {
				fmt.Fprintf(c.writer, "The default PUK %q is not supported.\n", DefaultPUK)
				continue
			}

			if !isPINLengthValid(newPUK) {
				fmt.Fprintf(c.writer, "PUK must be 6-8 characters long.\n")
				continue
			}

			pinAndPUK.PUK = newPUK
			pinAndPUK.PUKChanged = true
			break
		}
	}
	return pinAndPUK, nil
}

// ConfirmSlotOverwrite asks the user if the slot's private key and certificate can be overridden.
func (c *cliPrompt) ConfirmSlotOverwrite(ctx context.Context, message string, _ ContextualKeyInfo) (bool, error) {
	confirmation, err := prompt.Confirmation(ctx, c.writer, c.reader, message)
	return confirmation, trace.Wrap(err)
}
