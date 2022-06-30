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
	"fmt"
	"io"

	"github.com/gravitational/teleport/lib/utils/prompt"
)

// DefaultPrompt is a default implementation for LoginPrompt and
// RegistrationPrompt.
type DefaultPrompt struct {
	PINMessage                            string
	FirstTouchMessage, SecondTouchMessage string

	ctx   context.Context
	out   io.Writer
	count int
}

// NewDefaultPrompt creates a new default prompt.
// Default messages are suitable for login / authorization. Messages may be
// customized by setting the appropriate fields.
func NewDefaultPrompt(ctx context.Context, out io.Writer) *DefaultPrompt {
	return &DefaultPrompt{
		PINMessage:         "Enter your security key PIN",
		FirstTouchMessage:  "Tap your security key",
		SecondTouchMessage: "Tap your security key again to complete login",
		ctx:                ctx,
		out:                out,
	}
}

// PromptPIN prompts the user for a PIN.
func (p *DefaultPrompt) PromptPIN() (string, error) {
	return prompt.Password(p.ctx, p.out, prompt.Stdin(), p.PINMessage)
}

// PromptTouch prompts the user for a security key touch, using different
// messages for first and second prompts.
func (p *DefaultPrompt) PromptTouch() {
	if p.count == 0 {
		p.count++
		if p.FirstTouchMessage != "" {
			fmt.Fprintln(p.out, p.FirstTouchMessage)
		}
		return
	}
	if p.SecondTouchMessage != "" {
		fmt.Fprintln(p.out, p.SecondTouchMessage)
	}
}
