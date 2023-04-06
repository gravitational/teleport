package main

import (
	"context"
	"github.com/gravitational/teleport/lib/tbotv2"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/sirupsen/logrus"
	"time"
)

func main() {
	log := utils.NewLogger()
	err := run(log)
	if err != nil {
		log.WithError(err).Fatal("Bot exited with error :(")
	}
}

func run(log logrus.FieldLogger) error {
	ctx := context.Background()
	bot := tbotv2.NewBot(tbotv2.Config{
		AuthServer: "root.tele.ottr.sh:443",
		Store: &tbotv2.DirectoryStore{
			Path: "/Users/noahstride/code/gravitational/teleports/tbot-leaf-cluster/data",
		},
		Oneshot: true,
		Destinations: []tbotv2.Destination{
			&tbotv2.ApplicationDestination{
				Common: tbotv2.CommonDestination{
					TTL:   10 * time.Minute,
					Store: &tbotv2.DirectoryStore{Path: "./app-out"},
					Roles: []string{"access"},
				},
				Name: "httpbin",
			},
			&tbotv2.IdentityDestination{
				Common: tbotv2.CommonDestination{
					TTL:   10 * time.Minute,
					Roles: []string{"access"},
				},
			},
		},
	}, log)
	return bot.Run(ctx)
}
