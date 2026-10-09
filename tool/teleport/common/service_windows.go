// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"context"
	"os"
	"syscall"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows/svc"

	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/windowsservice"
)

func isWindowsService() (bool, error) {
	isSvc, err := svc.IsWindowsService()
	return isSvc, trace.Wrap(err)
}

// runAsWindowsService runs "teleport start" through the Windows Service
// Control Manager. The SCM launches the process and then waits for it to
// call StartServiceCtrlDispatcher and report svc.Running; without this, the
// SCM kills the process after its startup timeout (~30s) because it never
// hears back. windowsservice.Run provides that SCM handshake and cancels
// the context passed to Execute when the SCM sends a stop request, which
// windowsServiceHandler translates into Teleport's graceful shutdown.
func runAsWindowsService(clf config.CommandLineFlags, conf *servicecfg.Config) error {
	err := windowsservice.Run(&windowsservice.RunConfig{
		Name:    "teleport",
		Handler: &windowsServiceHandler{clf: clf, conf: conf},
		Logger:  conf.Logger,
	})
	return trace.Wrap(err, "running Teleport as a Windows service")
}

// windowsServiceHandler adapts the already-parsed "start" command flags and
// config to the windowsservice.ServiceHandler interface. Unlike lib/vnet's
// Windows service, which receives its arguments dynamically via
// mgr.Service.Start, the Teleport agent's service Arguments (set by the MSI)
// are static and were already parsed into clf/conf by common.Run before this
// handler is ever constructed, so Execute doesn't need to parse args itself.
type windowsServiceHandler struct {
	clf  config.CommandLineFlags
	conf *servicecfg.Config
}

// Execute implements windowsservice.ServiceHandler. ctx is canceled by the
// wrapper when the SCM asks the service to stop or the machine shuts down.
// This cancellation is translated into graceful shutdown by sending SIGTERM.
func (h *windowsServiceHandler) Execute(ctx context.Context, _ []string) error {
	logStart(ctx, h.clf, h.conf)
	sigC := make(chan os.Signal, 1)
	go func() {
		<-ctx.Done()
		sigC <- syscall.SIGTERM
	}()
	return trace.Wrap(service.RunWithSignalChannel(context.Background(), *h.conf, nil, sigC))
}
