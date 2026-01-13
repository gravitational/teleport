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
	"google.golang.org/protobuf/encoding/protojson"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	sshpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/ssh/v1"
)

// ValidatedMFAChallengeVerifier verifies that a validated MFA challenge exists in order to determine if the user has
// completed MFA.
type ValidatedMFAChallengeVerifier interface {
	VerifyValidatedMFAChallenge(ctx context.Context, req *mfav1.VerifyValidatedMFAChallengeRequest) error
}

// KeyboardInteractiveCallbackParams contains required parameters for KeyboardInteractiveCallback.
type KeyboardInteractiveCallbackParams struct {
	Metadata      ssh.ConnMetadata
	Challenge     ssh.KeyboardInteractiveChallenge
	Permissions   *ssh.Permissions
	Verifier      ValidatedMFAChallengeVerifier
	SourceCluster string
	Username      string
	Prompts       []*sshpb.AuthPrompt
}

// KeyboardInteractiveCallback implements an authentication layer on top of the SSH keyboard-interactive authentication
// method for SSH servers. It enables MFA after the user has already authenticated with a primary method (e.g., public
// key). Upon successful MFA verification, it returns the user's granted permissions. Intended for use with the
// x/crypto/ssh#ServerAuthCallbacks.KeyboardInteractiveCallback field.
func KeyboardInteractiveCallback(
	ctx context.Context,
	params KeyboardInteractiveCallbackParams,
) (*ssh.Permissions, error) {
	if err := checkKeyboardInteractiveCallbackParams(params); err != nil {
		return nil, trace.Wrap(err)
	}

	questions, echos, err := buildQuestions(params.Prompts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Send the auth prompts to the client and collect answers.
	answers, err := params.Challenge(params.Metadata.User(), "", questions, echos)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(answers) != len(questions) {
		return nil, trace.BadParameter("expected exactly %d answers, got %d answers", len(questions), len(answers))
	}

	// Process each answer from the client.
	for _, answer := range answers {
		resp := &sshpb.MFAPromptResponse{}

		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal([]byte(answer), resp); err != nil {
			return nil, trace.Wrap(err)
		}

		switch resp.GetResponse().(type) {
		case *sshpb.MFAPromptResponse_Reference:
			if err := handleMFAPromptResponse(
				ctx,
				resp,
				params.Verifier,
				params.Metadata.SessionID(),
				params.SourceCluster,
				params.Username,
			); err != nil {
				return nil, trace.Wrap(err)
			}

		case nil:
			return nil, trace.BadParameter("received sshpb.MFAPromptResponse with nil Response field")

		default:
			return nil, trace.BadParameter("received sshpb.AuthResponse with unknown response type")
		}
	}

	// Return the original permissions upon successful MFA verification to signal success.
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

	case params.Verifier == nil:
		return trace.BadParameter("params Verifier must be set")

	case params.SourceCluster == "":
		return trace.BadParameter("params SourceCluster must be set")

	case params.Username == "":
		return trace.BadParameter("params Username must be set")

	case len(params.Prompts) == 0:
		return trace.BadParameter("params Prompts must be set and contain at least one prompt")

	default:
		return nil
	}
}

func buildQuestions(prompts []*sshpb.AuthPrompt) ([]string, []bool, error) {
	// Marshal each prompt to JSON since the authentication method only supports UTF-8 strings.
	questions := make([]string, 0, len(prompts))

	for _, prompt := range prompts {
		bytes, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(prompt)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		questions = append(questions, string(bytes))
	}

	// All false by default for now since the only supported prompt type is MFA which doesn't require echoing.
	echos := make([]bool, len(prompts))

	return questions, echos, nil
}

func handleMFAPromptResponse(
	ctx context.Context,
	resp *sshpb.MFAPromptResponse,
	verifier ValidatedMFAChallengeVerifier,
	sessionID []byte,
	sourceCluster,
	username string,
) error {
	ref := resp.GetReference()
	if ref == nil {
		return trace.BadParameter("received sshpb.MFAPromptResponse with nil Reference field")
	}

	challengeName := ref.GetChallengeName()
	if challengeName == "" {
		return trace.BadParameter("received sshpb.MFAPromptResponseReference with empty ChallengeName field")
	}

	// Call the verifier to ensure the validated MFA challenge exists and is tied to the correct user and session.
	return trace.Wrap(
		verifier.VerifyValidatedMFAChallenge(
			ctx,
			&mfav1.VerifyValidatedMFAChallengeRequest{
				Name: challengeName,
				Payload: &mfav1.SessionIdentifyingPayload{
					Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
						SshSessionId: sessionID,
					},
				},
				SourceCluster: sourceCluster,
				Username:      username,
			},
		),
	)
}
