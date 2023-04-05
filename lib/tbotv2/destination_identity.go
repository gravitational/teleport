package tbotv2

import (
	"context"
	"github.com/gravitational/trace"
	"time"
)

type IdentityDestination struct {
}

func (d *IdentityDestination) Generate(ctx context.Context, bot BotI, store Store, roles []string, ttl time.Duration) error {
	id, err := bot.GenerateIdentity(ctx, IdentityRequest{
		roles: roles,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Persist to store
	// TODO: Write the whole shebang
	return trace.Wrap(store.Write(ctx, "identity", id.CertBytes))
}
