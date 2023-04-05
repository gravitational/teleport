package tbotv2

import (
	"context"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/trace"
	"time"
)

// Yeah yeah, we'll avoid this horrid interface
// BotI represents the bot locally, or the bot over gRPC and is used by the
// destinations to call necessary methods.
type BotI interface {
	GenerateIdentity(ctx context.Context, req IdentityRequest) (*identity.Identity, error)
	// ListenForRotation enables destinations to listen for a CA rotation
	ListenForRotation(ctx context.Context) (chan struct{}, func(), error)
	ClientForIdentity(ctx context.Context, id *identity.Identity) (auth.ClientI, error)
	// TODO: Should this provide everything or should we expect them to use their own client where appropriate ?
}

type Store interface {
	Write(ctx context.Context, name string, data []byte) error
	Read(ctx context.Context, name string) ([]byte, error)
}

type destWrapper struct {
	bot         BotI
	store       Store
	destination interface {
		Generate(ctx context.Context, bot BotI, store Store, roles []string, ttl time.Duration) error
	}
	Roles []string
	TTL   time.Duration
	Renew time.Duration

	// TODO: This could hold concept of "status":
	// e.g waiting for renewal, renewing
}

func (i *destWrapper) Run(ctx context.Context) error {
	rotationTrigger, stop, err := i.bot.ListenForRotation(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer stop()

	firstRenewal := make(chan struct{})
	firstRenewal <- struct{}{}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-firstRenewal:
		case <-rotationTrigger:
		case <-time.After(i.Renew):
			// TODO: Don't leak this timer lol
			// TODO: Retry logic
			// TODO: Backoff logic
			// TODO: Timeout logic
			err := i.destination.Generate(ctx, i.bot, i.store, i.Roles, i.TTL)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}
}

func (i *destWrapper) Oneshot(ctx context.Context) error {
	return trace.Wrap(i.destination.Generate(ctx, i.bot, i.store, i.Roles, i.TTL))
}
