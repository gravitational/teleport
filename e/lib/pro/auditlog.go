package pro

import (
	"context"
	"time"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	rclient "github.com/gravitational/reporting/client"
	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// AuditLog implements events.IAuditLog and extends the open-source implementation
// it is initialized with by anonymizing certain usage events and forwarding them
// to the control plane
type AuditLog struct {
	// AuditLogConfig is the audit log configuration
	AuditLogConfig
	// Entry is used for logging
	*log.Entry
}

// AuditLogConfig represents the audit log configuration
type AuditLogConfig struct {
	// Inner the audit log this logger wraps
	Inner events.IAuditLog
	// Recorder is the underlying recording client
	Recorder rclient.Client
	// Anonymizer is used for anonymizing sent data
	Anonymizer utils.Anonymizer
}

// Check checks that the audit log config is valid
func (c *AuditLogConfig) Check() error {
	if c.Inner == nil {
		return trace.BadParameter("missing Inner")
	}
	if c.Recorder == nil {
		return trace.BadParameter("missing Recorder")
	}
	if c.Anonymizer == nil {
		return trace.BadParameter("missing Anonymizer")
	}
	return nil
}

// NewAuditLog returns a new audit log for Teleport Pro
func NewAuditLog(config AuditLogConfig) (*AuditLog, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &AuditLog{
		AuditLogConfig: config,
		Entry: log.WithFields(log.Fields{
			trace.Component: "usage",
		}),
	}, nil
}

// EmitAuditEvent emits the specified event.
func (l *AuditLog) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	return trace.Wrap(l.Inner.EmitAuditEvent(ctx, event))
}

func (l *AuditLog) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	chunk, err := l.Inner.GetSessionChunk(namespace, sid, offsetBytes, maxBytes)
	return chunk, trace.Wrap(err)
}

func (l *AuditLog) GetSessionEvents(namespace string, sid session.ID, after int, includePrintEvents bool) ([]events.EventFields, error) {
	events, err := l.Inner.GetSessionEvents(namespace, sid, after, includePrintEvents)
	return events, trace.Wrap(err)
}

func (l *AuditLog) SearchEvents(fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, order apitypes.EventOrder, startKey string) ([]apievents.AuditEvent, string, error) {
	events, lastKey, err := l.Inner.SearchEvents(fromUTC, toUTC, namespace, eventTypes, limit, order, startKey)
	return events, lastKey, trace.Wrap(err)
}

func (l *AuditLog) SearchSessionEvents(fromUTC, toUTC time.Time, limit int, order apitypes.EventOrder, startKey string, cond *apitypes.WhereExpr) ([]apievents.AuditEvent, string, error) {
	events, lastKey, err := l.Inner.SearchSessionEvents(fromUTC, toUTC, limit, order, startKey, cond)
	return events, lastKey, trace.Wrap(err)
}

func (l *AuditLog) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	return l.Inner.StreamSessionEvents(ctx, sessionID, startIndex)
}

func (l *AuditLog) Close() error {
	return trace.Wrap(l.Inner.Close())
}
