package ex

import (
	"context"
	"github.com/gravitational/teleport/api/client"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

func run() error {
	ctx := context.Background()
	log := logrus.StandardLogger()

	credential := &config.ClientCredentialOutput{}
	bot := tbot.New(&config.BotConfig{
		AuthServer: "root.tele.ottr.sh:443",
		Onboarding: config.OnboardingConfig{
			TokenValue: "my-token",
			JoinMethod: types.JoinMethodKubernetes,
		},
		Outputs: []config.Output{
			credential,
		},
	}, log)

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return bot.Run(egCtx)
	})

	eg.Go(func() error {
		if err := credential.Wait(ctx); err != nil {
			return err
		}

		c, err := client.New(egCtx, client.Config{
			Addrs: []string{
				"root.tele.ottr.sh:443",
			},
			Credentials: []client.Credentials{credential},
		})
		if err != nil {
			return err
		}

		nodes, err := c.GetNodes(ctx, apidefaults.Namespace)
		if err != nil {
			return err
		}
		log.Infof("there are %d nodes", len(nodes))

		return nil
	})

	return eg.Wait()
}
