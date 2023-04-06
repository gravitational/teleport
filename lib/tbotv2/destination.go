package tbotv2

import (
	"context"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
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
}

type Store interface {
	Write(ctx context.Context, name string, data []byte) error
	Read(ctx context.Context, name string) ([]byte, error)
}

type CommonDestination struct {
	// Store requires polymorphic marshalling/unmarshalling
	Store Store         `yaml:"-"`
	Roles []string      `yaml:"roles"`
	TTL   time.Duration `yaml:"ttl"`
	Renew time.Duration `yaml:"renew"`
}

func (d *CommonDestination) UnmarshalYAML(node *yaml.Node) error {
	// Alias the type to get rid of the UnmarshalYAML :)
	type raw CommonDestination
	if err := node.Decode((*raw)(d)); err != nil {
		return trace.Wrap(err)
	}
	// We now have set up all the fields except those with special handling

	rawStore := struct {
		Store yaml.Node `yaml:"store"`
	}{}
	if err := node.Decode(&rawStore); err != nil {
		return trace.Wrap(err)
	}
	store, err := unmarshalStore(&rawStore.Store)
	if err != nil {
		return err
	}
	d.Store = store
	return nil
}

func (d *CommonDestination) Run(ctx context.Context, bot BotI, generate func(ctx context.Context, bot BotI) error) error {
	rotationTrigger, stop, err := bot.ListenForRotation(ctx)
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
		case <-time.After(d.Renew):
			// TODO: Don't leak this timer lol
			// TODO: Retry logic
			// TODO: Backoff logic
			// TODO: Timeout logic
			err := generate(ctx, bot)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}
}

func (d *CommonDestination) Oneshot(ctx context.Context, bot BotI, generate func(ctx context.Context, bot BotI) error) error {
	return trace.Wrap(generate(ctx, bot))
}
