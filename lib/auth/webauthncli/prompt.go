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

package webauthncli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth/touchid"
	"github.com/gravitational/teleport/lib/utils/prompt"
)

// DefaultPrompt is a default implementation for LoginPrompt and
// RegistrationPrompt.
type DefaultPrompt struct {
	PINMessage                            string
	FirstTouchMessage, SecondTouchMessage string
	AcknowledgeTouchMessage               string
	PromptCredentialMessage               string

	ctx context.Context
	out io.Writer

	count int
}

// NewDefaultPrompt creates a new default prompt.
// Default messages are suitable for login / authorization. Messages may be
// customized by setting the appropriate fields.
func NewDefaultPrompt(ctx context.Context, out io.Writer) *DefaultPrompt {
	return &DefaultPrompt{
		PINMessage:              "Enter your security key PIN",
		FirstTouchMessage:       "Tap your security key",
		SecondTouchMessage:      "Tap your security key again to complete login",
		AcknowledgeTouchMessage: "Detected security key tap",
		PromptCredentialMessage: "Choose the user for login",
		ctx:                     ctx,
		out:                     out,
	}
}

// PromptPIN prompts the user for a PIN.
func (p *DefaultPrompt) PromptPIN() (string, error) {
	return prompt.Password(p.ctx, p.out, prompt.Stdin(), p.PINMessage)
}

// PromptTouch prompts the user for a security key touch, using different
// messages for first and second prompts. Error is always nil.
func (p *DefaultPrompt) PromptTouch() (TouchAcknowledger, error) {
	if p.count == 0 {
		p.count++
		if p.FirstTouchMessage != "" {
			fmt.Fprintln(p.out, p.FirstTouchMessage)
		}
		return p.ackTouch, nil
	}
	if p.SecondTouchMessage != "" {
		fmt.Fprintln(p.out, p.SecondTouchMessage)
	}
	return p.ackTouch, nil
}

func (p *DefaultPrompt) ackTouch() error {
	fmt.Fprintln(p.out, p.AcknowledgeTouchMessage)
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
		numOrName, err := prompt.Input(p.ctx, p.out, prompt.Stdin(), p.PromptCredentialMessage)
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

type credentialPicker interface {
	PromptCredential([]*CredentialInfo) (*CredentialInfo, error)
}

// ToTouchIDCredentialPicker adapts a wancli credential picker, such as
// LoginPrompt or DefaultPrompt, to a touchid.CredentialPicker
func ToTouchIDCredentialPicker(p credentialPicker) touchid.CredentialPicker {
	return tidPickerAdapter{impl: p}
}

type tidPickerAdapter struct {
	impl credentialPicker
}

func (p tidPickerAdapter) PromptCredential(creds []*touchid.CredentialInfo) (*touchid.CredentialInfo, error) {
	credMap := make(map[*CredentialInfo]*touchid.CredentialInfo)
	wcreds := make([]*CredentialInfo, len(creds))
	for i, c := range creds {
		cred := &CredentialInfo{
			ID: []byte(c.CredentialID),
			User: UserInfo{
				UserHandle: c.User.UserHandle,
				Name:       c.User.Name,
			},
		}
		credMap[cred] = c
		wcreds[i] = cred
	}

	wchoice, err := p.impl.PromptCredential(wcreds)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	choice, ok := credMap[wchoice]
	if !ok {
		return nil, fmt.Errorf("prompt returned invalid credential: %#v", wchoice)
	}
	return choice, nil
}
