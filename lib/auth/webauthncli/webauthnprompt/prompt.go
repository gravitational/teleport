// Copyright 2022 Gravitational, Inc
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

package webauthnprompt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/prompt"
)

// LoginPrompt is the user interface for FIDO2Login.
//
// Prompts can have remote implementations, thus all methods may error.
type LoginPrompt interface {
	// PromptPIN prompts the user for their PIN.
	PromptPIN() (string, error)
	// PromptTouch prompts the user for a security key touch.
	// In certain situations multiple touches may be required (PIN-protected
	// devices, passwordless flows, etc).
	PromptTouch() error
	// PromptCredential prompts the user to choose a credential, in case multiple
	// credentials are available.
	// Callers are free to modify the slice, such as by sorting the credentials,
	// but must return one of the pointers contained within.
	PromptCredential(creds []*CredentialInfo) (*CredentialInfo, error)
}

// CredentialInfo holds information about a WebAuthn credential, typically a
// resident public key credential.
type CredentialInfo struct {
	ID   []byte
	User UserInfo
}

// UserInfo holds information about a credential owner.
type UserInfo struct {
	// UserHandle is the WebAuthn user handle (also referred as user ID).
	UserHandle []byte
	Name       string
}

// DefaultPrompt is a default implementation for LoginPrompt and
// RegistrationPrompt.
type DefaultPrompt struct {
	pinMessage                            string
	firstTouchMessage, secondTouchMessage string
	promptCredentialMessage               string

	ctx context.Context
	out io.Writer

	count int
}

type PromptOptions struct {
	HasTOTP          bool
	Quiet            bool
	WithDevicePrefix string
	Out              io.Writer
	// CustomLoginPrompt is used when you want to pass your custom login prompt.
	CustomLoginPrompt LoginPrompt
}

// NewDefaultPrompt creates a new default prompt.
// Default messages are suitable for login / authorization. Messages may be
// customized by setting PromptOptions.
func NewDefaultPrompt(ctx context.Context, opts PromptOptions) *DefaultPrompt {
	p := &DefaultPrompt{
		pinMessage:              "Enter your security key PIN",
		firstTouchMessage:       "Tap your security key",
		secondTouchMessage:      "Tap your security key again to complete login",
		promptCredentialMessage: "Choose the user for login",
		ctx:                     ctx,
		out:                     opts.Out,
	}

	if opts.Quiet {
		p.firstTouchMessage = ""
		p.secondTouchMessage = ""
		return p
	}

	if opts.WithDevicePrefix != "" {
		p.firstTouchMessage = fmt.Sprintf("Tap any %ssecurity key", opts.WithDevicePrefix)
		p.secondTouchMessage = fmt.Sprintf("Tap your %ssecurity key to complete login", opts.WithDevicePrefix)
	}

	if opts.HasTOTP {
		p.firstTouchMessage = fmt.Sprintf("Tap any %ssecurity key or enter a code from a %sOTP device", opts.WithDevicePrefix, opts.WithDevicePrefix)
	}

	return p
}

// PromptPIN prompts the user for a PIN.
func (p *DefaultPrompt) PromptPIN() (string, error) {
	return prompt.Password(p.ctx, p.out, prompt.Stdin(), p.pinMessage)
}

// PromptTouch prompts the user for a security key touch, using different
// messages for first and second prompts. Error is always nil.
func (p *DefaultPrompt) PromptTouch() error {
	if p.count == 0 {
		p.count++
		if p.firstTouchMessage != "" {
			fmt.Fprintln(p.out, p.firstTouchMessage)
		}
		return nil
	}
	if p.secondTouchMessage != "" {
		fmt.Fprintln(p.out, p.secondTouchMessage)
	}
	return nil
}

// PromptCredential prompts the user to choose a credential, in case multiple
// credentials are available.
func (p *DefaultPrompt) PromptCredential(creds []*CredentialInfo) (*CredentialInfo, error) {
	// Shouldn't happen, but let's check just in case.
	if len(creds) == 0 {
		return nil, errors.New("attempted to prompt credential with empty credentials")
	}

	sort.Slice(creds, func(i, j int) bool {
		c1 := creds[i]
		c2 := creds[j]
		return c1.User.Name < c2.User.Name
	})
	for i, cred := range creds {
		fmt.Fprintf(p.out, "[%v] %v\n", i+1, cred.User.Name)
	}

	for {
		numOrName, err := prompt.Input(p.ctx, p.out, prompt.Stdin(), p.promptCredentialMessage)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		switch num, err := strconv.Atoi(numOrName); {
		case err != nil: // See if a name was typed instead.
			for _, cred := range creds {
				if cred.User.Name == numOrName {
					return cred, nil
				}
			}
		case num >= 1 && num <= len(creds): // Valid number.
			return creds[num-1], nil
		}

		fmt.Fprintf(p.out, "Invalid user choice: %q\n", numOrName)
	}
}
