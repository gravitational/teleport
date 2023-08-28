/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client/mfa"
)

func runPresenceTask(ctx context.Context, term io.Writer, auth auth.ClientI, tc *TeleportClient, sessionID string) error {
	fmt.Fprintf(term, "\r\nTeleport > MFA presence enabled\r\n")

	ticker := time.NewTicker(mfaChallengeInterval)
	defer ticker.Stop()

	stream, err := auth.MaintainSessionPresence(ctx)
	if err != nil {
		fmt.Fprintf(term, "\r\nstream error: %v\r\n", err)
		return trace.Wrap(err)
	}

	for {
		select {
		case <-ticker.C:
			req := &proto.PresenceMFAChallengeSend{
				Request: &proto.PresenceMFAChallengeSend_ChallengeRequest{
					ChallengeRequest: &proto.PresenceMFAChallengeRequest{SessionID: sessionID},
				},
			}

			err = stream.Send(req)
			if err != nil {
				return trace.Wrap(err)
			}

			challenge, err := stream.Recv()
			if err != nil {
				return trace.Wrap(err)
			}

			solution, err := solveMFA(ctx, term, tc, challenge)
			if err != nil {
				return trace.Wrap(err)
			}

			req = &proto.PresenceMFAChallengeSend{
				Request: &proto.PresenceMFAChallengeSend_ChallengeResponse{
					ChallengeResponse: solution,
				},
			}

			err = stream.Send(req)
			if err != nil {
				return trace.Wrap(err)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func solveMFA(ctx context.Context, term io.Writer, tc *TeleportClient, challenge *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	fmt.Fprint(term, "\r\nTeleport > Please tap your MFA key\r\n")

	// This is here to enforce the usage of a MFA device.
	// We don't support TOTP for live presence.
	challenge.TOTP = nil

	response, err := tc.NewMFAPrompt(mfa.WithQuiet())(ctx, challenge)
	if err != nil {
		fmt.Fprintf(term, "\r\nTeleport > Failed to confirm presence: %v\r\n", err)
		return nil, trace.Wrap(err)
	}

	fmt.Fprint(term, "\r\nTeleport > Received MFA presence confirmation\r\n")
	return response, nil
}
