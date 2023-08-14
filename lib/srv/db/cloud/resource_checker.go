/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloud

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
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
	// ResourceMatchers is a list of database resource matchers.
	ResourceMatchers []services.ResourceMatcher
	// Clients is an interface for retrieving cloud clients.
	Clients cloud.Clients
	// Context is the database server close context.
	Context context.Context
	// Log is used for logging.
	Log logrus.FieldLogger
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
	if c.Context == nil {
		c.Context = context.Background()
	}
	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, teleport.ComponentDatabase)
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
