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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

func runPresenceTask(ctx context.Context, out io.Writer, auth auth.ClientI, tc *TeleportClient, sessionID string) error {
	err := utils.WriteAll(out.Write, []byte("\r\nTeleport > MFA presence enabled\r\n"))
	if err != nil {
		return trace.Wrap(err)
	}

	ticker := time.NewTicker(mfaChallengeInterval)
	stream, err := auth.MaintainSessionPresence(ctx)
	if err != nil {
		utils.WriteAll(out.Write, []byte(fmt.Sprintf("\r\nstream error: %v\r\n", err)))
		return trace.Wrap(err)
	}

outer:
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

			solution, err := solveMFA(ctx, out, tc, challenge)
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
			break outer
		}
	}

	return nil
}

func solveMFA(ctx context.Context, term io.Writer, tc *TeleportClient, challenge *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	utils.WriteAll(term.Write, []byte("\r\nTeleport > Please tap your MFA key within 15 seconds\r\n"))
	challenge.TOTP = nil

	response, err := PromptMFAChallenge(ctx, tc.Config.WebProxyAddr, challenge, "", true)
	if err != nil {
		utils.WriteAll(term.Write, []byte(fmt.Sprintf("\r\nTeleport > Failed to confirm presence: %v\r\n", err)))
		return nil, trace.Wrap(err)
	}

	utils.WriteAll(term.Write, []byte("\r\nTeleport > Received MFA presence confirmation\r\n"))
	return response, nil
}
