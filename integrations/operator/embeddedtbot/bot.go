package embeddedtbot

import (
	"context"
	"crypto/tls"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/integrations/operator/embeddedtbot/protectedclient"
	"github.com/gravitational/teleport/lib/tbot"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/utils"
)

type EmbeddedBot struct {
	cfg *BotConfig

	clientCache *protectedclient.Cache
}

func New(ctx context.Context, botConfig *BotConfig) (*EmbeddedBot, error) {
	err := botConfig.CheckAndSetDefaults(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bot := &EmbeddedBot{
		cfg: botConfig,
	}
	bot.clientCache = protectedclient.NewCache(bot.buildClient, func() ([]byte, error) {
		dest := bot.cfg.Outputs[0].GetDestination()
		return dest.Read(ctx, identity.TLSCertKey)
	})
	return bot, nil

}

func (b *EmbeddedBot) Preflight(ctx context.Context) (*proto.PingResponse, error) {
	b.cfg.Oneshot = true
	bot := tbot.New((*config.BotConfig)(b.cfg), log.StandardLogger())
	err := bot.Run(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	syncClient, release, err := b.clientCache.Get(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()

	pong, err := syncClient.Ping(ctx)
	return &pong, trace.Wrap(err)
}

func (b *EmbeddedBot) Run(ctx context.Context) error {
	b.cfg.Oneshot = false
	bot := tbot.New((*config.BotConfig)(b.cfg), log.StandardLogger())
	return trace.Wrap(bot.Run(ctx))
}

func (b *EmbeddedBot) GetSyncClient(ctx context.Context) (*protectedclient.ProtectedClient, func(), error) {
	return b.clientCache.Get(ctx)
}

// buildClient reads tbot's memory disttination, retrieves the certificates
// and builds a new Teleport client using those certs.
func (b *EmbeddedBot) buildClient(ctx context.Context) (*protectedclient.ProtectedClient, error) {
	log.Infof("Building a new client to connect to %s", b.cfg.AuthServer)
	storageDestination := b.cfg.Storage.Destination

	// Hack to be able to reuse LoadIdentity functions from tbot
	// LoadIdentity expects to have all the artifacts required for a bot
	// We loop over missing artifacts and are loading them from the bot storage to the destination
	for _, artifact := range identity.GetArtifacts() {
		if artifact.Kind == identity.KindBotInternal {
			value, err := storageDestination.Read(ctx, artifact.Key)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if err := b.cfg.Outputs[0].GetDestination().Write(ctx, artifact.Key, value); err != nil {
				return nil, trace.Wrap(err)
			}

		}
	}

	id, err := identity.LoadIdentity(ctx, b.cfg.Outputs[0].GetDestination(), identity.BotKinds()...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c, err := client.New(ctx, client.Config{
		Addrs:       []string{b.cfg.AuthServer},
		Credentials: []client.Credentials{clientCredentials{id}},
	})
	return protectedclient.NewClient(c), trace.Wrap(err)
}

type clientCredentials struct {
	id *identity.Identity
}

func (c clientCredentials) Dialer(client.Config) (client.ContextDialer, error) {
	return nil, trace.NotImplemented("no dialer")
}

func (c clientCredentials) TLSConfig() (*tls.Config, error) {
	return c.id.TLSConfig(utils.DefaultCipherSuites())
}

func (c clientCredentials) SSHClientConfig() (*ssh.ClientConfig, error) {
	return c.id.SSHClientConfig(false)
}
