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

package redis

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/healthcheck"
	"github.com/gravitational/teleport/lib/srv/db/healthchecks"
)

// NewHealthChecker resolves an endpoint from DB URI.
func NewHealthChecker(_ context.Context, cfg healthchecks.HealthCheckerConfig) (healthcheck.HealthChecker, error) {
	connOpts, err := getConnectionOptions(cfg.Database)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	hostPort := getHostPort(connOpts)
	return healthcheck.NewTargetDialer(func(context.Context) ([]string, error) {
		return []string{hostPort}, nil
	}), nil
}
