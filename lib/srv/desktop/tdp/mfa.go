package tdp

import (
	"context"
	"errors"
	"log/slog"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/trace"
)

var (
	ErrUnexpectedTDPMessageType = errors.New("unexpected message type")
)

// convertChallenge converts an MFA challenge to a Message. Returns
// a non-nil error if the conversion fails
type convertChallenge func(*proto.MFAAuthenticateChallenge) (Message, error)

// isMFAResponse returns:
//   - ErrUnexpectedTDPMessageType if a valid messages was received but was not an MFA message.
//   - Any other non-nil error if there was an error intepreeting the message.
//   - nil if a valid, non-nil MFA messages was found.
type isMFAResponse func(Message) (*proto.MFAAuthenticateResponse, error)

// newMfaPrompt constructs a function that reads, encodes, and sends an MFA challenge to the client,
// then waits for the corresponding MFA response message. It caches any non-MFA messages received so
// that they may be forwarded to the server later on.
func NewMfaPrompt(rw MessageReadWriter, isResponse isMFAResponse, toMessage convertChallenge, withheld *[]Message, log *slog.Logger) mfa.PromptFunc {
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

			resp, err := isResponse(msg)
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
