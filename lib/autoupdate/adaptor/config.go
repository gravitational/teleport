/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package adaptor

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/utils"
)

// ProcessSupervisor is a subset of the functions we need to register our services
// and access the inventory/auth client (used to export auth maintenance windows).
// `lib/service/service.go` is growing too large but I don't have the time to do a full
// refactoring. This interface allows us to put the updater detection code in a separate
// package and not contribute to the mess service.TeleportProcess is.
type ProcessSupervisor interface {
	// RegisterFunc creates a service from function spec and registers
	// it within the system
	RegisterFunc(name string, fn func() error)

	// RegisterCriticalFunc creates a critical service from function spec and registers
	// it within the system, if this service exits with error,
	// the process shuts down.
	RegisterCriticalFunc(name string, fn func() error)

	OnExit(serviceName string, callback func(any))
}

type UpgradeWindowsClient interface {
	ExportUpgradeWindows(ctx context.Context, req proto.ExportUpgradeWindowsRequest) (proto.ExportUpgradeWindowsResponse, error)
	Ping(ctx context.Context) (proto.PingResponse, error)
}

type Config struct {
	ResolverAddr utils.NetAddr
	HostUUID     string
	Supervisor   ProcessSupervisor
	Log          *slog.Logger
	ClientGetter func() (UpgradeWindowsClient, error)
	Sentinel     <-chan inventory.DownstreamSender
}

func (c *Config) Check(ctx context.Context) error {
	if c.ResolverAddr.String() == "" {
		return trace.BadParameter("resolver address is not set, this is a bug")
	}
	if c.HostUUID == "" {
		return trace.BadParameter("host UUID is not set, this is a bug")
	}
	if c.Supervisor == nil {
		return trace.BadParameter("supervisor is not set, this is a bug")
	}
	if c.Log == nil {
		return trace.BadParameter("logger is not set, this is a bug")
	}
	if c.ClientGetter == nil {
		return trace.BadParameter("client getter is not set, this is a bug")
	}
	if c.Sentinel == nil {
		return trace.BadParameter("connection sentinel is not set, this is a bug")
	}
	return nil
}
