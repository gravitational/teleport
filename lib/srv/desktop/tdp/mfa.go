// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package tdp

import (
	"context"
	"errors"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
)

var (
	ErrUnexpectedTDPMessageType = errors.New("unexpected message type")
)

// convertChallenge converts an MFA challenge to a Message. Returns
// a non-nil error if the conversion fails
type convertChallenge func(*proto.MFAAuthenticateChallenge) (Message, error)

// asMFAResponse returns:
//   - ErrUnexpectedTDPMessageType if a valid messages was received but was not an MFA message.
//   - Any other non-nil error if there was an error interpreting the message.
//   - nil if a valid, non-nil MFA messages was found.
type asMFAResponse func(Message) (*proto.MFAAuthenticateResponse, error)

// NewMfaPrompt constructs a function that reads, encodes, and sends an MFA challenge to the client,
// then waits for the corresponding MFA response message. It caches any non-MFA messages received so
// that they may be forwarded to the server later on.
func NewMFAPrompt(rw MessageReadWriter, asResponse asMFAResponse, toMessage convertChallenge, withheld *[]Message, log *slog.Logger) mfa.PromptFunc {
	return func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
		challengeMsg, err := toMessage(chal)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		log.DebugContext(ctx, "Writing MFA challenge to client")
		if err = rw.WriteMessage(challengeMsg); err != nil {
			return nil, trace.Wrap(err)
		}

		for {
			msg, err := rw.ReadMessage()
			if err != nil {
				return nil, trace.Wrap(err)
			}

			resp, err := asResponse(msg)
			if err != nil {
				if errors.Is(err, ErrUnexpectedTDPMessageType) {
					// Withhold this non-MFA message and try reading again
					log.DebugContext(ctx, "Received non-MFA message", "message", msg)
					*withheld = append(*withheld, msg)
					continue
				} else {
					log.DebugContext(ctx, "Error receiving MFA response", "error", err)
					// Unexpected error occurred while inspecting the message
					return nil, trace.Wrap(err)
				}
			}
			// Found our MFA response!
			log.DebugContext(ctx, "Received MFA response")
			return resp, nil
		}
	}
}
