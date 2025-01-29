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

package cloud

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/services"
)

// DiscoveryResourceChecker defines an interface for checking database
// resources created by the discovery service.
type DiscoveryResourceChecker interface {
	// Check performs required checks on provided database resource before it
	// gets registered.
	Check(ctx context.Context, database types.Database) error
}

// DiscoveryResourceCheckerConfig is the config for DiscoveryResourceChecker.
type DiscoveryResourceCheckerConfig struct {
	// AWSConfigProvider provides [aws.Config] for AWS SDK service clients.
	AWSConfigProvider awsconfig.Provider
	// ResourceMatchers is a list of database resource matchers.
	ResourceMatchers []services.ResourceMatcher
	// Clients is an interface for retrieving cloud clients.
	Clients cloud.Clients
	// Context is the database server close context.
	Context context.Context
	// Logger is used for logging.
	Logger *slog.Logger
}

// CheckAndSetDefaults validates the config and sets default values.
func (c *DiscoveryResourceCheckerConfig) CheckAndSetDefaults() error {
	if c.Clients == nil {
		cloudClients, err := cloud.NewClients()
		if err != nil {
			return trace.Wrap(err)
		}
		c.Clients = cloudClients
	}
	if c.AWSConfigProvider == nil {
		return trace.BadParameter("missing AWSConfigProvider")
	}
	if c.Context == nil {
		c.Context = context.Background()
	}
	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, teleport.ComponentDatabase)
	}
	return nil
}

// NewDiscoveryResourceChecker creates a new DiscoveryResourceChecker.
func NewDiscoveryResourceChecker(cfg DiscoveryResourceCheckerConfig) (DiscoveryResourceChecker, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	credentialsChecker, err := newCrednentialsChecker(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(greedy52) implement name checker.
	return &discoveryResourceChecker{
		checkers: []DiscoveryResourceChecker{
			newURLChecker(cfg),
			credentialsChecker,
		},
	}, nil
}

// discoveryResourceChecker is a composite checker.
type discoveryResourceChecker struct {
	checkers []DiscoveryResourceChecker
}

// Check calls Check from all its checkers and aggregate the errors.
func (c *discoveryResourceChecker) Check(ctx context.Context, database types.Database) error {
	if database.Origin() != types.OriginCloud {
		return nil
	}

	errors := make([]error, 0, len(c.checkers))
	for _, checker := range c.checkers {
		errors = append(errors, trace.Wrap(checker.Check(ctx, database)))
	}
	return trace.NewAggregate(errors...)
}
