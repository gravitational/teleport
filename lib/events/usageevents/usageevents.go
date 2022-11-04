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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// UsageLogger is a trivial audit log sink that forwards an anonymized subset of
// audit log events to Teleport.
type UsageLogger struct {
	// Entry is a log entry
	*log.Entry

	// inner is a wrapped audit log implementation
	inner events.IAuditLog

	// reporter is a usage reporter, where filtered audit events will be sent
	reporter services.UsageReporter
}

// report submits a usage event, but silently ignores events if no reporter is
// configured.
func (u *UsageLogger) report(event services.UsageAnonymizable) {
	if u.reporter == nil {
		return
	}

	u.reporter.SubmitAnonymizedUsageEvents(event)
}

func (u *UsageLogger) filterAuditEvent(ctx context.Context, event apievents.AuditEvent) {
	switch e := event.(type) {
	case *apievents.UserLogin:
		// Only count successful logins.
		if !e.Success {
			return
		}

		// Note: we can have different behavior based on event code (local vs
		// SSO) if desired, but we currently only care about connector type /
		// method
		u.report(&services.UsageUserLogin{
			UserName:      e.User,
			ConnectorType: e.Method,
		})
	case *apievents.SessionStart:
		// Note: session.start is only SSH.
		u.report(&services.UsageSessionStart{
			UserName:    e.User,
			SessionType: string(types.SSHSessionKind),
			Interactive: true, // TODO: we don't know this in SessionStart
		})
	}
}

func (u *UsageLogger) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	u.filterAuditEvent(ctx, event)

	if u.inner != nil {
		return u.inner.EmitAuditEvent(ctx, event)
	}

	// Note: we'll consider this implemented.
	return nil
}

// New creates a new usage event IAuditLog impl, which wraps another IAuditLog
// impl and forwards a subset of audit log events to the cluster UsageReporter
// service.
func New(reporter services.UsageReporter, inner events.IAuditLog) (*UsageLogger, error) {
	l := log.WithFields(log.Fields{
		trace.Component: teleport.Component(teleport.ComponentUsageReporting),
	})

	return &UsageLogger{
		Entry:    l,
		reporter: reporter,
		inner:    inner,
	}, nil
}
