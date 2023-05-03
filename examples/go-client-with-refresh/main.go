package main

import (
	"context"
	"fmt"
	"github.com/gravitational/teleport/api/client"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/examples/go-client-with-refresh/dynamic"
	teleUtils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
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

	log := teleUtils.NewLogger()
	if err := run(ctx, log); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log logrus.FieldLogger) error {
	proxyAddr := os.Getenv("PROXY_ADDR")                    // e.g noah.teleport.sh:443
	identityFilePath := os.Getenv("TELEPORT_IDENTITY_FILE") // e.g ./identity-file
	clusterName := os.Getenv("CLUSTER_NAME")                // e.g noah.teleport.sh

	cred, err := dynamic.NewDynamicIdentityFile(identityFilePath, clusterName)
	go func() {
		// This goroutine loop could be replaced with a file watcher.
		for {
			time.Sleep(time.Second * 30)
			if err := cred.Reload(); err != nil {
				log.WithError(err).Warn("Failed to reload identity file")
				continue
			}
			log.Info("Successfully reloaded identity file from disk. New client connections will use this identity.")
		}
	}()

	cfg := client.Config{
		Addrs: []string{proxyAddr},
		Credentials: []client.Credentials{
			cred,
		},
		DialOpts: []grpc.DialOption{
			// Provides better feedback on connection errors
			grpc.WithReturnConnectionError(),
		},
		// ALPNSNIAuthDialClusterName allows the client to connect to the
		// auth server through the proxy.
		ALPNSNIAuthDialClusterName: clusterName,
	}
	clt, err := client.New(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clt.Close()

	return monitorLoop(ctx, log, clt)
}

// This loop replicates some work that needs to run continously against the
// Teleport API and have access to an up to date client.
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
			log.WithError(err).Error("failed to fetch nodes")
		} else {
			log.WithFields(logrus.Fields{
				"count":    len(nodes),
				"duration": time.Since(start),
			}).Info("Fetched nodes list")
		}

		time.Sleep(5 * time.Second)
	}
}
