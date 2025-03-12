/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package teleterm

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/apiserver"
	"github.com/gravitational/teleport/lib/teleterm/clusteridcache"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/daemon"
)

// Serve starts daemon service
func Serve(ctx context.Context, cfg Config) error {
	var hardwareKeyPromptConstructor func(clusterURI uri.ResourceURI) keys.HardwareKeyPrompt
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	grpcCredentials, err := createGRPCCredentials(cfg.Addr, cfg.CertsDir)
	if err != nil {
		return trace.Wrap(err)
	}

	clock := clockwork.NewRealClock()

	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                cfg.HomeDir,
		Clock:              clock,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		AddKeysToAgent:     cfg.AddKeysToAgent,
		HardwareKeyPromptConstructor: func(rootClusterURI uri.ResourceURI) keys.HardwareKeyPrompt {
			return hardwareKeyPromptConstructor(rootClusterURI)
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	clusterIDCache := &clusteridcache.Cache{}

	daemonService, err := daemon.New(daemon.Config{
		Storage:                         storage,
		CreateTshdEventsClientCredsFunc: grpcCredentials.tshdEvents,
		PrehogAddr:                      cfg.PrehogAddr,
		KubeconfigsDir:                  cfg.KubeconfigsDir,
		AgentsDir:                       cfg.AgentsDir,
		ClusterIDCache:                  clusterIDCache,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(gzdunek): Move tshdEventsClient out of daemonService so that we can
	// construct the prompt before creating Storage.
	hardwareKeyPromptConstructor = daemonService.NewHardwareKeyPromptConstructor
	apiServer, err := apiserver.New(apiserver.Config{
		HostAddr:           cfg.Addr,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		Daemon:             daemonService,
		Storage:            storage,
		TshdServerCreds:    grpcCredentials.tshd,
		ListeningC:         cfg.ListeningC,
		ClusterIDCache:     clusterIDCache,
		InstallationID:     cfg.InstallationID,
		Clock:              clock,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	serverAPIWait := make(chan error)
	go func() {
		err := apiServer.Serve()
		serverAPIWait <- err
	}()

	// Wait for shutdown signals
	go func() {
		shutdownSignals := []os.Signal{os.Interrupt, syscall.SIGTERM}
		c := make(chan os.Signal, len(shutdownSignals))
		signal.Notify(c, shutdownSignals...)

		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "Context closed, stopping service")
		case sig := <-c:
			slog.InfoContext(ctx, "Captured signal, stopping service", "signal", sig)
		}

		daemonService.Stop()
		apiServer.Stop()
	}()

	errAPI := <-serverAPIWait

	if errAPI != nil {
		return trace.Wrap(errAPI, "shutting down due to API Server error")
	}

	return nil
}

type grpcCredentials struct {
	tshd       grpc.ServerOption
	tshdEvents daemon.CreateTshdEventsClientCredsFunc
}

func createGRPCCredentials(tshdServerAddress string, certsDir string) (*grpcCredentials, error) {
	shouldUseMTLS := strings.HasPrefix(tshdServerAddress, "tcp://")

	if !shouldUseMTLS {
		return &grpcCredentials{
			tshd: grpc.Creds(nil),
			tshdEvents: func() (grpc.DialOption, error) {
				return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
			},
		}, nil
	}

	rendererCertPath := filepath.Join(certsDir, rendererCertFileName)
	mainProcessCertPath := filepath.Join(certsDir, mainProcessCertFileName)
	tshdCertPath := filepath.Join(certsDir, tshdCertFileName)
	tshdKeyPair, err := generateAndSaveCert(tshdCertPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tshdCreds, err := createServerCredentials(
		tshdKeyPair,
		// Client certs will be read on an incoming connection.  The client setup in the Electron app is
		// orchestrated in a way where the client saves its cert to disk before initiating a connection.
		[]string{rendererCertPath, mainProcessCertPath},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// To create client creds for tshd events service, we need to read the server cert. However, at
	// this point we'd need to wait for the Electron app to save the cert under rendererCertPath.
	//
	// Instead of waiting for it, we're going to capture the logic in a function that's going to be
	// called after the Electron app calls UpdateTshdEventsServerAddress of the Terminal service.
	// Since this calls the gRPC server hosted by tsh, we can assume that by this point the Electron
	// app has saved the cert to disk – without the cert, it wouldn't be able to call the tsh server.
	createTshdEventsClientCredsFunc := func() (grpc.DialOption, error) {
		creds, err := createClientCredentials(tshdKeyPair, rendererCertPath)
		return creds, trace.Wrap(err, "could not create tshd events client credentials")
	}

	return &grpcCredentials{
		tshd:       tshdCreds,
		tshdEvents: createTshdEventsClientCredsFunc,
	}, nil
}
