// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package vnet

import (
	"context"
	"log/slog"
	"os"

	"github.com/Microsoft/go-winio"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
)

const (
	pipePath = `\\.\pipe\vnet`
)

// UserProcessConfig provides the necessary configuration to run VNet.
type UserProcessConfig struct {
	// AppProvider is a required field providing an interface implementation for [AppProvider].
	AppProvider AppProvider
	// ClusterConfigCache is an optional field providing [ClusterConfigCache]. If empty, a new cache
	// will be created.
	ClusterConfigCache *ClusterConfigCache
	// HomePath is the tsh home used for Teleport clients created by VNet. Resolved using the same
	// rules as HomeDir in tsh.
	HomePath string
}

func (c *UserProcessConfig) CheckAndSetDefaults() error {
	if c.AppProvider == nil {
		return trace.BadParameter("missing AppProvider")
	}
	if c.HomePath == "" {
		c.HomePath = profile.FullProfilePath(os.Getenv(types.HomeEnvVar))
	}
	return nil
}

// RunUserProcess launches a Windows service in the background that in turn
// calls [RunAdminProcess]. The user process exposes a gRPC interface on a named
// pipe that the admin process uses to query application names and get user
// certificates for apps.
//
// RunUserProcess returns a [ProcessManager] which controls the lifecycle of
// both the user and admin processes.
//
// The caller is expected to call Close on the process manager to clean up any
// resources and terminate the admin process, which will in turn stop the
// networking stack and deconfigure the host OS.
//
// ctx is used to wait for setup steps that happen before RunUserProcess hands out the
// control to the process manager. If ctx gets canceled during RunUserProcess, the process
// manager gets closed along with its background tasks.
func RunUserProcess(ctx context.Context, config *UserProcessConfig) (pm *ProcessManager, err error) {
	defer func() {
		if pm != nil && err != nil {
			pm.Close()
		}
	}()
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	ipv6Prefix, err := NewIPv6Prefix()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dnsIPv6 := ipv6WithSuffix(ipv6Prefix, []byte{2})
	// By default only the LocalSystem account, administrators, and the owner of
	// the current process can access the pipe. The admin service runs as the
	// LocalSystem account. We don't leak anything by letting processes owned
	// by the same user as this process to connect to the pipe, they could read
	// TELEPORT_HOME anyway.
	pipe, err := winio.ListenPipe(pipePath, &winio.PipeConfig{})
	if err != nil {
		return nil, trace.Wrap(err, "listening on named pipe")
	}
	pm, processCtx := newProcessManager()
	pm.AddCriticalBackgroundTask("pipe closer", func() error {
		<-processCtx.Done()
		return trace.Wrap(pipe.Close())
	})
	pm.AddCriticalBackgroundTask("admin process", func() error {
		adminConfig := AdminProcessConfig{
			NamedPipe:  pipePath,
			IPv6Prefix: ipv6Prefix.String(),
			DNSAddr:    dnsIPv6.String(),
			HomePath:   config.HomePath,
		}
		return trace.Wrap(execAdminProcess(processCtx, adminConfig))
	})
	pm.AddCriticalBackgroundTask("ipc service", func() error {
		// TODO(nklaassen): wrap [config.AppProvider] with a gRPC service to expose
		// the necessary methods to the admin process over [pipe].
		// For now just accept and drop any connections to prove the admin
		// process can dial the pipe. The pipe will be closed when the process
		// context completes and any blocked Accept call will return with an error.
		slog.InfoContext(processCtx, "Listening on named pipe", "pipe", pipe.Addr().String())
		for {
			_, err := pipe.Accept()
			if err != nil {
				return trace.Wrap(err)
			}
		}
	})
	return pm, nil
}
