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
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

// UsageLogger is a trivial audit log sink that forwards an anonymized subset of
// audit log events to Teleport.
type UsageLogger struct {
	// Entry is a log entry
	*logrus.Entry

	// inner is a wrapped audit log implementation
	inner apievents.Emitter

	// reporter is a usage reporter, where filtered audit events will be sent
	reporter services.UsageReporter
}

// report submits a usage event, but silently ignores events if no reporter is
// configured.
func (u *UsageLogger) report(event services.UsageAnonymizable) error {
	if u.reporter == nil {
		return nil
	}

	return trace.Wrap(u.reporter.SubmitAnonymizedUsageEvents(event))
}

func (u *UsageLogger) reportAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	switch e := event.(type) {
	case *apievents.UserLogin:
		// Only count successful logins.
		if !e.Success {
			return nil
		}

		// Note: we can have different behavior based on event code (local vs
		// SSO) if desired, but we currently only care about connector type /
		// method
		return trace.Wrap(u.report(&services.UsageUserLogin{
			UserName:      e.User,
			ConnectorType: e.Method,
		}))
	case *apievents.SessionStart:
		// Note: session.start is only SSH.
		return trace.Wrap(u.report(&services.UsageSessionStart{
			UserName:    e.User,
			SessionType: string(types.SSHSessionKind),
		}))
	case *apievents.GithubConnectorCreate:
		return trace.Wrap(u.report(&services.UsageSSOCreate{
			ConnectorType: types.KindGithubConnector,
		}))
	case *apievents.OIDCConnectorCreate:
		return trace.Wrap(u.report(&services.UsageSSOCreate{
			ConnectorType: types.KindOIDCConnector,
		}))
	case *apievents.SAMLConnectorCreate:
		return trace.Wrap(u.report(&services.UsageSSOCreate{
			ConnectorType: types.KindSAMLConnector,
		}))
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
func New(reporter services.UsageReporter, log logrus.FieldLogger, inner events.IAuditLog) (*UsageLogger, error) {
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
