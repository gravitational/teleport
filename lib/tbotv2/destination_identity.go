package tbotv2

import (
	"context"
	"github.com/gravitational/trace"
)

var IdentityDestinationType = "identity"

type IdentityDestination struct {
	Common CommonDestination `yaml:",inline"`
}

func (d *IdentityDestination) String() string {
	return d.Common.String(IdentityDestinationType)
}

func (d *IdentityDestination) CheckAndSetDefaults() error {
	return trace.Wrap(d.Common.CheckAndSetDefaults())
}

func (d *IdentityDestination) Oneshot(ctx context.Context, bot DestinationHost) error {
	return trace.Wrap(d.Generate(ctx, bot))
}

func (d *IdentityDestination) Run(ctx context.Context, bot DestinationHost) error {
	return trace.Wrap(d.Common.Run(ctx, bot, d.Generate))
}

func (d *IdentityDestination) Generate(ctx context.Context, bot DestinationHost) error {
	id, err := bot.GenerateIdentity(ctx, IdentityRequest{
		ttl:   d.Common.TTL,
		roles: d.Common.Roles,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Persist to store
	// TODO: Write the whole identity file rather than the summary
	// The point is this works.
	return trace.Wrap(d.Common.Store.Write(ctx, "identity", []byte(id.String())))
}
