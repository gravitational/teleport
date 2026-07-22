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
	"slices"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/encoding/protojson"

	sshpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/ssh/v1"
)

// MFACeremonyPerformer performs a session-bound MFA ceremony with the user and returns the solved challenge name.
type MFACeremonyPerformer func(ctx context.Context, sessionID []byte) (challengeName string, err error)

// AuthCallbackConfig holds configuration for in-band authentication callbacks.
type AuthCallbackConfig struct {
	// MFAPerformer is called when the server signals partial success for publickey + keyboard-interactive auth. If nil,
	// the client will fall back to its default auth methods and will not perform in-band MFA.
	MFAPerformer MFACeremonyPerformer
}

// AuthCallback returns an ssh.ClientAuthCallback that dynamically selects an auth method based on the server's response
// during the handshake. When the server signals partial success for publickey, it offers keyboard-interactive to
// perform additional auth (e.g., in-band MFA). Return (nil, nil) otherwise to let the client fall back to its default
// auth methods.
func AuthCallback(ctx context.Context, config AuthCallbackConfig) ssh.ClientAuthCallback {
	return func(authCtx *ssh.ClientAuthContext) (ssh.AuthMethod, error) {
		// We use keyboard-interactive auth exclusively for when some auth-related data needs to be communicated to the
		// client and back (e.g., in-band MFA). The partial success with publickey + keyboard-interactive pattern is
		// unique to this flow and will not occur during normal auth. We offer it only once, skipping if
		// keyboard-interactive has already been tried.
		if config.MFAPerformer != nil &&
			slices.Contains(authCtx.PartialSuccessMethods, "publickey") &&
			slices.Contains(authCtx.AllowedMethods, "keyboard-interactive") &&
			!slices.Contains(authCtx.TriedMethods, "keyboard-interactive") {
			return KeyboardInteractive(ctx, config.MFAPerformer, authCtx.Metadata), nil
		}

		// Returning nil, nil tells the SSH client there is no additional auth method to offer for this server response
		// and fallback to the default behavior of trying the next auth method in the list.
		return nil, nil
	}
}

// KeyboardInteractive returns an ssh.AuthMethod that performs any additional verification requested by the server via
// the keyboard-interactive authentication method.
func KeyboardInteractive(ctx context.Context, p MFACeremonyPerformer, m ssh.ConnMetadata) ssh.AuthMethod {
	return ssh.KeyboardInteractive(
		func(_ string, _ string, questions []string, _ []bool) ([]string, error) {
			answers := make([]string, 0, len(questions))

			// Handle each question from the server before returning all answers.
			for _, question := range questions {
				pr := &sshpb.AuthPrompt{}

				if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal([]byte(question), pr); err != nil {
					return nil, trace.Wrap(err)
				}

				if pr.GetPrompt() == nil {
					return nil, trace.BadParameter("received sshpb.AuthPrompt with nil Prompt field")
				}

				switch pr.WhichPrompt() {
				case sshpb.AuthPrompt_MfaPrompt_case:
					answer, err := handleMFAPrompt(ctx, p, m)
					if err != nil {
						return nil, trace.Wrap(err)
					}
					answers = append(answers, answer)

				default:
					return nil, trace.BadParameter("invalid or unsupported auth prompt type: %T", pr)
				}
			}

			return answers, nil
		},
	)
}

// handleMFAPrompt returns an answer to a keyboard-interactive question requiring MFA by performing a session-bound MFA
// ceremony with the user. The answer is a JSON-marshaled sshpb.MFAPromptResponse referencing the solved challenge.
func handleMFAPrompt(ctx context.Context, p MFACeremonyPerformer, m ssh.ConnMetadata) (string, error) {
	name, err := p(ctx, m.SessionID())
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Construct the response referencing the challenge solved by the user.
	resp := sshpb.MFAPromptResponse_builder{
		Reference: sshpb.MFAPromptResponseReference_builder{
			ChallengeName: name,
		}.Build(),
	}.Build()

	// Marshal the response to JSON since the authentication method only supports UTF-8 strings.
	answerBytes, err := protojson.Marshal(resp)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return string(answerBytes), nil
}
