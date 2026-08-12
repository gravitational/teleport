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

const (
	// DefaultPresenceMaxDuration is the default max duration that a moderated session
	// can continue between presence verifications.
	DefaultPresenceMaxDuration = time.Minute
)

// PresenceMaintainer allows maintaining presence with the Auth service.
type PresenceMaintainer interface {
	MaintainSessionPresence(ctx context.Context) (proto.AuthService_MaintainSessionPresenceClient, error)
}

// presenceOptions allows passing optional overrides to RunPresenceTask.
type presenceOptions struct {
	Clock clockwork.Clock
}

// PresenceOption is a functional option for RunPresenceTask.
type PresenceOption func(*presenceOptions)

// WithPresenceClock sets the clock to be used by RunPresenceTask.
func WithPresenceClock(clock clockwork.Clock) PresenceOption {
	return func(p *presenceOptions) {
		p.Clock = clock
	}
}

// RunDefaultPresenceTask performs an MFA ceremony every 30 seconds to detect that a user is
// still present and attentive.
func RunDefaultPresenceTask(ctx context.Context, term io.Writer, maintainer PresenceMaintainer, sessionID string, baseCeremony *mfa.Ceremony, opts ...PresenceOption) error {
	return RunPresenceTask(ctx, term, maintainer, sessionID, baseCeremony, DefaultPresenceMaxDuration/2, opts...)
}

// RunPresenceTask periodically performs and MFA ceremony to detect that a user is
// still present and attentive.
func RunPresenceTask(ctx context.Context, term io.Writer, maintainer PresenceMaintainer, sessionID string, baseCeremony *mfa.Ceremony, interval time.Duration, opts ...PresenceOption) error {
	fmt.Fprintf(term, "\r\nTeleport > MFA presence enabled\r\n")

	o := &presenceOptions{
		Clock: clockwork.NewRealClock(),
	}
	for _, opt := range opts {
		opt(o)
	}

	ticker := o.Clock.NewTicker(interval)
	defer ticker.Stop()

	stream, err := maintainer.MaintainSessionPresence(ctx)
	if err != nil {
		fmt.Fprintf(term, "\r\nstream error: %v\r\n", err)
		return trace.Wrap(err)
	}

	presenceCeremony := &mfa.Ceremony{
		MFACeremonyConstructor: baseCeremony.MFACeremonyConstructor,
		PromptConstructor: func(opts ...mfa.PromptOpt) mfa.Prompt {
			return mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
				// Replace normal output with terminal messages specific to moderated sessions.
				opts = append(opts, mfa.WithQuiet())

				fmt.Fprint(term, "\r\nTeleport > Please tap your MFA key\r\n")

				mfaResp, err := baseCeremony.PromptConstructor(opts...).Run(ctx, chal)
				if err != nil {
					fmt.Fprintf(term, "\r\nTeleport > Failed to confirm presence: %v\r\n", err)
					return nil, trace.Wrap(err)
				}

				fmt.Fprint(term, "\r\nTeleport > Received MFA presence confirmation\r\n")
				return mfaResp, nil
			})
		},
		CreateAuthenticateChallenge: func(ctx context.Context, chalReq *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
			req := &proto.PresenceMFAChallengeSend{
				Request: &proto.PresenceMFAChallengeSend_ChallengeRequest{
					ChallengeRequest: &proto.PresenceMFAChallengeRequest{
						SessionID:                sessionID,
						SSOClientRedirectURL:     chalReq.SSOClientRedirectURL,
						BrowserMFATSHRedirectURL: chalReq.BrowserMFATSHRedirectURL,
						ProxyAddress:             chalReq.ProxyAddress,
					},
				},
			}

			if err := stream.Send(req); err != nil {
				return nil, trace.Wrap(err)
			}

			challenge, err := stream.Recv()
			if err != nil {
				return nil, trace.Wrap(err)
			}

			// This is here to enforce the usage of a MFA device.
			// We don't support TOTP for live presence.
			challenge.TOTP = nil

			return challenge, nil
		},
	}

	for {
		select {
		case <-ticker.Chan():
			mfaResp, err := presenceCeremony.Run(ctx, &proto.CreateAuthenticateChallengeRequest{})
			if err != nil {
				return trace.Wrap(err)
			}

			resp := &proto.PresenceMFAChallengeSend{
				Request: &proto.PresenceMFAChallengeSend_ChallengeResponse{
					ChallengeResponse: mfaResp,
				},
			}

			err = stream.Send(resp)
			if err != nil {
				return trace.Wrap(err)
			}
		case <-ctx.Done():
			return nil
		}
	}
}
