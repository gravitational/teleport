/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package ssh

import (
	"context"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// PromptVerifier defines an interface for marshaling authentication prompts and verifying answers.
type PromptVerifier interface {
	// MarshalPrompt marshals the prompt to a UTF-8 encoded string and returns the echo flag.
	MarshalPrompt() (prompt string, echo bool, err error)

	// VerifyAnswer verifies the UTF-8 encoded answer provided by the client.
	VerifyAnswer(ctx context.Context, answer string) error
}

// KeyboardInteractiveCallbackParams contains required parameters for KeyboardInteractiveCallback.
type KeyboardInteractiveCallbackParams struct {
	Metadata        ssh.ConnMetadata
	Challenge       ssh.KeyboardInteractiveChallenge
	Permissions     *ssh.Permissions
	PromptVerifiers []PromptVerifier
}

// KeyboardInteractiveCallback implements an extra authentication layer on top of the keyboard-interactive
// authentication method for SSH servers after the user has already authenticated with a primary method (e.g., public
// key). The callback presents one or more prompts to the client, as defined by the provided PromptVerifier
// implementations, and verifies the user's answers. Upon successful verification of all prompts, the user's granted
// permissions are returned. Intended for use with the x/crypto/ssh#ServerAuthCallbacks.KeyboardInteractiveCallback
// field.
func KeyboardInteractiveCallback(
	ctx context.Context,
	params KeyboardInteractiveCallbackParams,
) (*ssh.Permissions, error) {
	if err := checkKeyboardInteractiveCallbackParams(params); err != nil {
		return nil, trace.Wrap(err)
	}

	// Prepare the questions and echo flags for each PromptVerifier.
	questions := make([]string, 0, len(params.PromptVerifiers))
	echos := make([]bool, 0, len(params.PromptVerifiers))

	for _, verifier := range params.PromptVerifiers {
		question, echo, err := verifier.MarshalPrompt()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		questions = append(questions, question)
		echos = append(echos, echo)
	}

	// Send the questions to the client and collect answers.
	answers, err := params.Challenge(params.Metadata.User(), "", questions, echos)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(answers) != len(questions) {
		return nil, trace.BadParameter("expected exactly %d answer(s), got %d answer(s)", len(questions), len(answers))
	}

	// Process each answer using its corresponding PromptVerifier.
	for i, answer := range answers {
		if err := params.PromptVerifiers[i].VerifyAnswer(ctx, answer); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Return the original permissions upon successful verification to signal success.
	return params.Permissions, nil
}

func checkKeyboardInteractiveCallbackParams(params KeyboardInteractiveCallbackParams) error {
	switch {
	case params.Metadata == nil:
		return trace.BadParameter("params Metadata must be set")

	case params.Challenge == nil:
		return trace.BadParameter("params Challenge must be set")

	case params.Permissions == nil:
		return trace.BadParameter("params Permissions must be set")

	case len(params.PromptVerifiers) == 0:
		return trace.BadParameter("params PromptVerifiers must be set and contain at least one PromptVerifier")

	default:
		return nil
	}
}
