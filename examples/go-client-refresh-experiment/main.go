package main

import (
	"context"
	"github.com/gravitational/teleport/api/client"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"os"
	"os/signal"
	"time"
)

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		unix.SIGTERM,
		unix.SIGINT,
	)
	defer cancel()

	log := utils.NewLogger()
	if err := run(ctx, log); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, log logrus.FieldLogger) error {
	proxyAddr := os.Getenv("PROXY_ADDR")
	identityFilePath := os.Getenv("TELEPORT_IDENTITY_FILE")

	cfg := client.Config{
		Addrs: []string{proxyAddr},
		Credentials: []client.Credentials{
			client.LoadIdentityFile(identityFilePath),
		},
	}
	clt, err := client.New(ctx, cfg)
	if err != nil {
		return trace.Wrap(err, "creating client")
	}
	defer clt.Close()

	return monitorLoop(ctx, log, clt)
}

func monitorLoop(
	ctx context.Context,
	log logrus.FieldLogger,
	clt *client.Client,
) error {
	for {
		// Exit is context is cancelled.
		if err := ctx.Err(); err != nil {
			log.Info(
				"Detected context cancellation, exiting watch loop!",
			)
			return nil
		}

		// This action represents any unary action against the Teleport API
		start := time.Now()
		nodes, err := clt.GetNodes(ctx, apidefaults.Namespace)
		if err != nil {
			return trace.Wrap(err)
		}

		log.WithFields(logrus.Fields{
			"count":    len(nodes),
			"duration": time.Since(start),
		}).Info("Fetched nodes list")

		time.Sleep(5 * time.Second)
	}
}
