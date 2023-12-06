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

package client

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
)

// PresenceMaintainer allows maintaining presence with the Auth service.
type PresenceMaintainer interface {
	MaintainSessionPresence(ctx context.Context) (proto.AuthService_MaintainSessionPresenceClient, error)
}

const mfaChallengeInterval = time.Second * 30

// presenceOptions allows passing optional overrides
// to RunPresenceTask. Mainly used by tests.
type presenceOptions struct {
	Clock clockwork.Clock
}

// PresenceOption a functional option for RunPresenceTask.
type PresenceOption func(p *presenceOptions)

// WithPresenceClock sets the clock to be used by RunPresenceTask.
func WithPresenceClock(clock clockwork.Clock) PresenceOption {
	return func(p *presenceOptions) {
		p.Clock = clock
	}
}

// RunPresenceTask periodically performs and MFA ceremony to detect that a user is
// still present and attentive.
func RunPresenceTask(ctx context.Context, term io.Writer, maintainer PresenceMaintainer, sessionID string, mfaPrompt mfa.Prompt, opts ...PresenceOption) error {
	fmt.Fprintf(term, "\r\nTeleport > MFA presence enabled\r\n")

	o := &presenceOptions{
		Clock: clockwork.NewRealClock(),
	}

	for _, opt := range opts {
		opt(o)
	}

	ticker := o.Clock.NewTicker(mfaChallengeInterval)
	defer ticker.Stop()

	stream, err := maintainer.MaintainSessionPresence(ctx)
	if err != nil {
		fmt.Fprintf(term, "\r\nstream error: %v\r\n", err)
		return trace.Wrap(err)
	}

	for {
		select {
		case <-ticker.Chan():
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

			fmt.Fprint(term, "\r\nTeleport > Please tap your MFA key\r\n")

			// This is here to enforce the usage of a MFA device.
			// We don't support TOTP for live presence.
			challenge.TOTP = nil

			solution, err := mfaPrompt.Run(ctx, challenge)
			if err != nil {
				fmt.Fprintf(term, "\r\nTeleport > Failed to confirm presence: %v\r\n", err)
				return trace.Wrap(err)
			}

			fmt.Fprint(term, "\r\nTeleport > Received MFA presence confirmation\r\n")

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
