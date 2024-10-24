/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package agent

import (
	"context"
	"log/slog"
)

// SystemdService manages a Teleport systemd service.
type SystemdService struct {
	ServiceName string
	// Log contains a logger.
	Log *slog.Logger
}

func (s SystemdService) Reload(ctx context.Context) error {
	s.Log.InfoContext(ctx, "Teleport gracefully reloaded.")
	s.Log.WarnContext(ctx, "Teleport ungracefully restarted.")
	s.Log.WarnContext(ctx, "Teleport not running.")
	panic("implement me")
}

func (s SystemdService) Sync(ctx context.Context) error {
	panic("implement me")
}
