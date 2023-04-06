package tbotv2

import (
	"context"
	"fmt"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"time"
)

// DestinationHost represents the thing "hosting" a destination plugin. This will
// usually the be the Bot itself, or, the standalone destination host which will
// communicate with the Bot daemon over gRPC.
type DestinationHost interface {
	GenerateIdentity(ctx context.Context, req IdentityRequest) (*identity.Identity, error)
	// ListenForRotation enables destinations to listen for a CA rotation
	ListenForRotation(ctx context.Context) (chan struct{}, func(), error)
	ClientForIdentity(ctx context.Context, id *identity.Identity) (auth.ClientI, error)
	Logger() logrus.FieldLogger
}

type Destination interface {
	Run(ctx context.Context, bot DestinationHost) error
	Oneshot(ctx context.Context, bot DestinationHost) error
	CheckAndSetDefaults() error
	String() string
}

type CommonDestination struct {
	// Store requires polymorphic marshalling/unmarshalling
	Store Store         `yaml:"-"`
	Roles []string      `yaml:"roles"`
	TTL   time.Duration `yaml:"ttl"`
	Renew time.Duration `yaml:"renew"`
}

func (d *CommonDestination) String(destinationType string) string {
	return fmt.Sprintf("%s (%s)", destinationType, d.Store.String())
}

func (d *CommonDestination) CheckAndSetDefaults() error {
	if d.TTL == 0 {
		d.TTL = time.Minute * 10
	}
	return nil
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

func (d *CommonDestination) Run(ctx context.Context, bot DestinationHost, generate func(ctx context.Context, bot DestinationHost) error) error {
	rotationTrigger, stop, err := bot.ListenForRotation(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	bot.Logger().Info("Listening for rotations")
	defer stop()

	firstRenewal := make(chan struct{}, 1)
	firstRenewal <- struct{}{}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-firstRenewal:
		case <-rotationTrigger:
		case <-time.After(d.Renew):
			bot.Logger().Info("Renewing")
			// TODO: Don't leak this timer lol
			// TODO: Retry logic
			// TODO: Backoff logic
			// TODO: Timeout logic
			err := generate(ctx, bot)
			bot.Logger().Info("Renewal complete")
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}
}

func (d *CommonDestination) Oneshot(ctx context.Context, bot DestinationHost, generate func(ctx context.Context, bot DestinationHost) error) error {
	return trace.Wrap(generate(ctx, bot))
}
