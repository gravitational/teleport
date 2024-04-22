// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package log

import (
	"context"
	"log/slog"
	"slices"
	"sync/atomic"
)

type packageHandler struct {
	args   []any
	meta   []metadata
	logger atomic.Pointer[slog.Logger]
}

type metadata struct {
	group string
	attrs []slog.Attr
}

// NewPackageLogger creates a [slog.Logger] that defers setting
// any groups or attributes until the first use of the logger.
// This allows package global loggers to be created prior to
// any custom [slog.Handler] can be set via [slog.Default] AND
// still respect the formatting of the default handler set at
// runtime.
func NewPackageLogger(args ...any) *slog.Logger {
	return slog.New(&packageHandler{args: args})
}

// Enabled returns whether the provided level will be included in output.
func (d *packageHandler) Enabled(ctx context.Context, level slog.Level) bool {
	logger := d.getLogger()
	return logger.Enabled(ctx, level)
}

func (d *packageHandler) getLogger() *slog.Logger {
	logger := d.logger.Load()
	if logger != nil {
		return logger
	}
	logger = slog.With(d.args...)
	for _, goa := range d.meta {
		if goa.group != "" {
			logger = logger.WithGroup(goa.group)
		}

		for _, attr := range goa.attrs {
			logger = logger.With(attr)
		}
	}

	if d.logger.CompareAndSwap(nil, logger) {
		return logger
	}

	return d.getLogger()
}

// Handle formats the provided record and writes the underlying logger.
func (d *packageHandler) Handle(ctx context.Context, record slog.Record) error {
	logger := d.getLogger()
	return logger.Handler().Handle(ctx, record)
}

// WithAttrs clones the current handler with the provided attributes
// added to any existing attributes. If the underlying logger has not yet been
// set, then the attributes are stored so that they can later be expanded.
func (d *packageHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return d
	}

	logger := d.logger.Load()
	if logger != nil {
		return logger.Handler().WithAttrs(attrs)
	}

	return &packageHandler{
		args: slices.Clone(d.args),
		meta: append(slices.Clone(d.meta), metadata{attrs: attrs}),
	}
}

// WithGroup opens a new group. If the underlying logger has not yet been
// set, then the group is stored so that they can later be expanded.
func (d *packageHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return d
	}

	logger := d.logger.Load()
	if logger != nil {
		return logger.Handler().WithGroup(name)
	}

	return &packageHandler{
		args: slices.Clone(d.args),
		meta: append(slices.Clone(d.meta), metadata{group: name}),
	}
}
