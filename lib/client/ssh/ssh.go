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
	"fmt"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/encoding/protojson"

	sshpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/ssh/v1"
)

// MFACeremonyPerformer performs a session-bound MFA ceremony with the user and returns the solved challenge name.
type MFACeremonyPerformer interface {
	PerformSessionMFACeremony(ctx context.Context, sessionID []byte) (challengeName string, err error)
}

// KeyboardInteractive returns an ssh.AuthMethod that performs any additional verification requested by the server via
// the keyboard-interactive authentication method. This method is intended to be used with the
// x/crypto/ssh#ClientConfig.AuthCallback field as proposed in https://github.com/golang/go/issues/76146.
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

				switch pr.GetPrompt().(type) {
				case *sshpb.AuthPrompt_MfaPrompt:
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
	fmt.Println("Handling k-i question")
	name, err := p.PerformSessionMFACeremony(ctx, m.SessionID())
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Construct the response referencing the challenge solved by the user.
	resp := &sshpb.MFAPromptResponse{
		Response: &sshpb.MFAPromptResponse_Reference{
			Reference: &sshpb.MFAPromptResponseReference{
				ChallengeName: name,
			},
		},
	}

	// Marshal the response to JSON since the authentication method only supports UTF-8 strings.
	answerBytes, err := protojson.Marshal(resp)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return string(answerBytes), nil
}
