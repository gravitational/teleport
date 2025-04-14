/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

package accessmonitoring

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/accessmonitoring"
	"github.com/gravitational/teleport/lib/accessmonitoring/review"
	"github.com/gravitational/teleport/lib/backend"
)

// Client aggregates the parts of Teleport API client interface
// (as implemented by github.com/gravitational/teleport/api/client.Client)
// that are used by the access plugins.
type Client interface {
	types.Events
	SubmitAccessReview(ctx context.Context, params types.AccessReviewSubmission) (types.AccessRequest, error)
	ListAccessMonitoringRulesWithFilter(ctx context.Context, req *accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error)
	GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error)
}

// Config specifies the access monitoring service configuration.
type Config struct {
	// Logger is the logger for the access monitoring serivce.
	Logger *slog.Logger

	// Backend should be a backend.Backend which can be used for obtaining the
	// lock required to run the service.
	Backend backend.Backend

	// Client is the auth service client interface.
	Client Client
}

// CheckAndSetDefaults checks and sets default config values.
func (c *Config) CheckAndSetDefaults() error {
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	if c.Backend == nil {
		return trace.BadParameter("backend: must be non-nil")
	}
	if c.Client == nil {
		return trace.BadParameter("client: must be non-nil")
	}
	return nil
}

// AccessMonitoringService monitors access events and applies access monitoring
// rules.
type AccessMonitoringService struct {
	cfg Config
}

// NewAccessMonitoringSerivce returns a new access monitoring service.
func NewAccessMonitoringService(cfg Config) (*AccessMonitoringService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "failed to validate access monitoring service config")
	}
	return &AccessMonitoringService{
		cfg: cfg,
	}, nil
}

// Run the access monitoring service.
func (s *AccessMonitoringService) Run(ctx context.Context) (err error) {
	accessReviewHandler, err := review.NewHandler(review.Config{
		Logger:      s.cfg.Logger,
		HandlerName: types.BuiltInAutomaticReview,
		Client:      s.cfg.Client,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	monitor, err := accessmonitoring.NewAccessMonitor(accessmonitoring.Config{
		Logger:  s.cfg.Logger,
		Backend: s.cfg.Backend,
		Events:  s.cfg.Client,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Configure access review handlers.
	monitor.AddAccessMonitoringRuleHandler(accessReviewHandler.HandleAccessMonitoringRule)
	monitor.AddAccessRequestHandler(accessReviewHandler.HandleAccessRequest)

	return trace.Wrap(monitor.Run(ctx))
}
