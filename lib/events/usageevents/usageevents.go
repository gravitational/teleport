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

package usageevents

import (
	"cmp"
	"context"
	"log/slog"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

// UsageLogger is a trivial audit log sink that forwards an anonymized subset of
// audit log events to Teleport.
type UsageLogger struct {
	// logger emits log messages
	logger *slog.Logger

	// inner is a wrapped audit log implementation
	inner apievents.Emitter

	// reporter is a usage reporter, where filtered audit events will be sent
	reporter usagereporter.UsageReporter
}

var _ apievents.Emitter = (*UsageLogger)(nil)

// reportAuditEvent tries to convert the audit event into a usage event, and
// submits the usage event if successful, but silently ignores events if no
// reporter is configured.
func (u *UsageLogger) reportAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	if u.reporter == nil {
		return nil
	}

	if a := usagereporter.ConvertAuditEvent(event); a != nil {
		u.reporter.AnonymizeAndSubmit(a)
	}

	return nil
}

func (u *UsageLogger) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	ctx = context.WithoutCancel(ctx)
	if err := u.reportAuditEvent(ctx, event); err != nil {
		// We don't ever want this to fail or bubble up errors, so the best we
		// can do is complain to the logs.
		u.logger.WarnContext(ctx, "Failed to filter audit event", "error", err)
	}

	if u.inner != nil {
		return u.inner.EmitAuditEvent(ctx, event)
	}

	return nil
}

// New creates a new usage event IAuditLog impl, which wraps another IAuditLog
// impl and forwards a subset of audit log events to the cluster UsageReporter
// service.
func New(reporter usagereporter.UsageReporter, log *slog.Logger, inner apievents.Emitter) (*UsageLogger, error) {
	logger := cmp.Or(log, slog.Default())
	return &UsageLogger{
		logger:   logger.With(teleport.ComponentKey, teleport.ComponentUsageReporting),
		reporter: reporter,
		inner:    inner,
	}, nil
}
