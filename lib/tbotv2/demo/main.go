package main

import (
	"context"
	"github.com/gravitational/teleport/lib/tbotv2"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/sirupsen/logrus"
)

func main() {
	log := utils.NewLogger()
	err := run(log)
	if err != nil {
		log.WithError(err).Fatal("bot died")
	}
}

func run(log logrus.FieldLogger) error {
	log.Info("Running")
	ctx := context.Background()
	bot := tbotv2.NewBot(tbotv2.Config{
		AuthServer: "root.tele.ottr.sh:443",
		Dir:        "/Users/noahstride/code/gravitational/teleports/tbot-leaf-cluster/data",
		Oneshot:    true,
	}, log)
	log.Info("Bot created, starting")
	return bot.Run(ctx)
}
