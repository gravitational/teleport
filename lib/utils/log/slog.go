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

package log

import (
	"context"
	"log/slog"
)

type delegatingHandler struct {
	handler slog.Handler
}

func (d *delegatingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return d.handler.Enabled(ctx, level)
}

func (d *delegatingHandler) Handle(ctx context.Context, record slog.Record) error {
	return d.handler.Handle(ctx, record)
}

func (d *delegatingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
}

func (d *delegatingHandler) WithGroup(name string) slog.Handler {
	//TODO implement me
	panic("implement me")
}

func setSharedHandler(handler slog.Handler) {
	slog.Default()
}

func SharedSlog() *slog.Logger {
	return nil
}
