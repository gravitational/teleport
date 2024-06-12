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

package apiserver

import (
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/teleterm/daemon"
	"github.com/gravitational/teleport/lib/utils"
)

// Config is the APIServer configuration
type Config struct {
	// HostAddr is the APIServer host address
	HostAddr string
	// Daemon is the terminal daemon service
	Daemon *daemon.Service
	// Log is a component logger
	Log             logrus.FieldLogger
	TshdServerCreds grpc.ServerOption
	// ListeningC propagates the address on which the gRPC server listens. Mostly useful in tests, as
	// the Electron app gets the server port from stdout.
	ListeningC chan<- utils.NetAddr
}

// CheckAndSetDefaults checks and sets default config values.
func (c *Config) CheckAndSetDefaults() error {
	if c.HostAddr == "" {
		return trace.BadParameter("missing HostAddr")
	}

	if c.HostAddr == "" {
		return trace.BadParameter("missing certs dir")
	}

	if c.Daemon == nil {
		return trace.BadParameter("missing daemon service")
	}

	if c.TshdServerCreds == nil {
		return trace.BadParameter("missing TshdServerCreds")
	}

	if c.Log == nil {
		c.Log = logrus.WithField(teleport.ComponentKey, "conn:apiserver")
	}

	return nil
}
