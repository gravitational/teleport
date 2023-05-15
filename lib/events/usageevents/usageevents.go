/*
Copyright 2022 Gravitational, Inc.

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

package usageevents

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

// UsageLogger is a trivial audit log sink that forwards an anonymized subset of
// audit log events to Teleport.
type UsageLogger struct {
	// Entry is a log entry
	*logrus.Entry

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
	if err := u.reportAuditEvent(ctx, event); err != nil {
		// We don't ever want this to fail or bubble up errors, so the best we
		// can do is complain to the logs.
		u.Warnf("Failed to filter audit event: %+v", err)
	}

	if u.inner != nil {
		return u.inner.EmitAuditEvent(ctx, event)
	}

	return nil
}

// New creates a new usage event IAuditLog impl, which wraps another IAuditLog
// impl and forwards a subset of audit log events to the cluster UsageReporter
// service.
func New(reporter usagereporter.UsageReporter, log logrus.FieldLogger, inner apievents.Emitter) (*UsageLogger, error) {
	if log == nil {
		log = logrus.StandardLogger()
	}

	return &UsageLogger{
		Entry: log.WithField(
			trace.Component,
			teleport.Component(teleport.ComponentUsageReporting),
		),
		reporter: reporter,
		inner:    inner,
	}, nil
}
