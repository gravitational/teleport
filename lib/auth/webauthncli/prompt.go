/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package webauthncli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/auth/touchid"
)

// DefaultPrompt is a default implementation for LoginPrompt and
// RegistrationPrompt.
type DefaultPrompt struct {
	PINMessage                            string
	FirstTouchMessage, SecondTouchMessage string
	AcknowledgeTouchMessage               string
	PromptCredentialMessage               string

	// StdinFunc allows tests to override prompt.Stdin().
	// If nil prompt.Stdin() is used.
	StdinFunc func() prompt.StdinReader

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

func (p *DefaultPrompt) stdin() prompt.StdinReader {
	if p.StdinFunc == nil {
		return prompt.Stdin()
	}
	return p.StdinFunc()
}

// PromptPIN prompts the user for a PIN.
func (p *DefaultPrompt) PromptPIN() (string, error) {
	return prompt.Password(p.ctx, p.out, p.stdin(), p.PINMessage)
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
		numOrName, err := prompt.Input(p.ctx, p.out, p.stdin(), p.PromptCredentialMessage)
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
