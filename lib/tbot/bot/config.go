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

	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/bot/onboarding"
	"github.com/gravitational/trace"
	"github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"gopkg.in/yaml.v3"
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

// UnmarshalConfigContext is passed to the UnmarshalConfig method implemented by service
// config structs. It provides a way to unmarshal destinations without needing
// to maintain a "registry" using package-global variables or import them all
// in the service package.
type UnmarshalConfigContext interface {
	// ExtractDestination performs surgery on yaml.Node to unmarshal a
	// destination and then remove key/values from the yaml.Node that specify
	// the destination. This *hack* allows us to have the bot.Destination as a
	// field directly on an Output without needing a struct to wrap it for
	// polymorphic unmarshaling.
	ExtractDestination(node *yaml.Node) (destination.Destination, error)

	// UnmarshalDestination unmarshals a destination by looking at its "type
	// header" to determine which destination type it is.
	UnmarshalDestination(node *yaml.Node) (destination.Destination, error)
}
