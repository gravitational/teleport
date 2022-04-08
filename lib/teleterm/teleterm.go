// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package teleterm

import (
	"context"
	"os"
	"os/signal"

	"github.com/gravitational/teleport/lib/teleterm/apiserver"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/daemon"

	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

// Start starts daemon service
func Start(ctx context.Context, cfg Config) error {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                cfg.HomeDir,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	daemonService, err := daemon.New(daemon.Config{
		Storage:            storage,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	apiServer, err := apiserver.New(apiserver.Config{
		HostAddr: cfg.Addr,
		Daemon:   daemonService,
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
		c := make(chan os.Signal, len(cfg.ShutdownSignals))
		signal.Notify(c, cfg.ShutdownSignals...)
		select {
		case <-ctx.Done():
			log.Info("Context closed, stopping service.")
		case sig := <-c:
			log.Infof("Captured %s, stopping service.", sig)
		}
		daemonService.Stop()
		apiServer.Stop()
	}()

	log.Infof("tsh daemon is listening on %v.", cfg.Addr)

	errAPI := <-serverAPIWait

	if errAPI != nil {
		return trace.Wrap(errAPI, "shutting down due to API Server error")
	}

	return nil
}
