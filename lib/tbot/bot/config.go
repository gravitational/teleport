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

package bot

import (
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"

	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/bot/onboarding"
	"github.com/gravitational/teleport/lib/tbot/internal"
)

// Config contains the core bot's configuration. The tbot binary's configuration
// file format is handled by the lib/tbot/config package.
type Config struct {
	// Connection controls how the bot connects to the cluster.
	Connection connection.Config

	// Onboarding controls how the bot authenticates and joins the cluster.
	Onboarding onboarding.Config

	// InternalStorage is the destination to which the bot's internal state and
	// certificates will be written.
	InternalStorage destination.Destination

	// CredentialLifetime controls the TTL and renewal interval of the bot's
	// internal credentials.
	CredentialLifetime CredentialLifetime

	// FIPS controls whether the bot will run in a mode designed to comply with
	// Federal Information Processing Standards.
	FIPS bool

	// Logger to which errors and messages will be written.
	Logger *slog.Logger

	// ReloadCh can be used to trigger a reload its certificates, etc.
	ReloadCh <-chan struct{}

	// Services contains constructors that will be called to create the bot's
	// services.
	Services []ServiceBuilder

	// ClientMetrics will be used to record the bot's API client metrics.
	ClientMetrics *prometheus.ClientMetrics
}

// CheckAndSetDefaults validates the configuration and sets any default values.
func (c *Config) CheckAndSetDefaults() error {
	if err := c.Connection.Validate(); err != nil {
		return trace.Wrap(err, "validating connection")
	}
	if c.InternalStorage == nil {
		c.InternalStorage = destination.NewMemory()
	}
	if c.CredentialLifetime.IsEmpty() {
		c.CredentialLifetime = DefaultCredentialLifetime
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return nil
}

// UnmarshalConfigContext is passed to the UnmarshalConfig method implemented by
// service config structs. It provides a way to unmarshal fields that may be
// dynamically registered (like the Kubernetes Secret Destination, which is only
// available if you import the k8s package) without maintaining a global registry.
type UnmarshalConfigContext = internal.UnmarshalConfigContext
