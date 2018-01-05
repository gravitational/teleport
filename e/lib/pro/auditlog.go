package pro

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"time"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"

	rclient "github.com/gravitational/reporting/client"
	"github.com/gravitational/reporting/types"
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
	// AnonymizeKey is used for anonymizing sent data
	AnonymizeKey string
}

// Check checks that the audit log config is valid
func (c *AuditLogConfig) Check() error {
	if c.Inner == nil {
		return trace.BadParameter("missing Inner")
	}
	if c.Recorder == nil {
		return trace.BadParameter("missing Recorder")
	}
	if c.AnonymizeKey == "" {
		return trace.BadParameter("missing AnonymizeKey")
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

// EmitAuditEvent sends the anonymized usage metrics to the control plane and
// then calls EmitAuditEvent on the logger it wraps
func (l *AuditLog) EmitAuditEvent(eventType string, fields events.EventFields) error {
	switch eventType {
	case events.UserLoginEvent:
		l.Debugf("Recoding audit event %q.", eventType)
		l.Recorder.Record(types.NewUserLoginEvent(
			l.anonymize(fields.GetString(events.EventUser))))
	case events.SessionStartEvent:
		l.Debugf("Recoding audit event %q.", eventType)
		l.Recorder.Record(types.NewServerLoginEvent(
			fields.GetString(events.SessionServerID)))
	default:
		l.Debugf("Ignoring event %q.", eventType)
	}
	return trace.Wrap(l.Inner.EmitAuditEvent(eventType, fields))
}

func (l *AuditLog) PostSessionSlice(slice events.SessionSlice) error {
	for _, chunk := range slice.Chunks {
		if chunk.EventType == events.SessionStartEvent {
			var fields events.EventFields
			if err := json.Unmarshal(chunk.Data, &fields); err != nil {
				log.Warningf("Failed to unmarshal event: %v.", err)
			} else {
				l.Debugf("Recoding session event %q.", chunk.EventType)
				l.Recorder.Record(types.NewServerLoginEvent(
					fields.GetString(events.SessionServerID)))
			}
		}
	}
	return trace.Wrap(l.Inner.PostSessionSlice(slice))
}

func (l *AuditLog) PostSessionChunk(namespace string, sid session.ID, reader io.Reader) error {
	return trace.Wrap(l.Inner.PostSessionChunk(namespace, sid, reader))
}

func (l *AuditLog) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	chunk, err := l.Inner.GetSessionChunk(namespace, sid, offsetBytes, maxBytes)
	return chunk, trace.Wrap(err)
}

func (l *AuditLog) GetSessionEvents(namespace string, sid session.ID, after int) ([]events.EventFields, error) {
	events, err := l.Inner.GetSessionEvents(namespace, sid, after)
	return events, trace.Wrap(err)
}

func (l *AuditLog) SearchEvents(fromUTC, toUTC time.Time, query string) ([]events.EventFields, error) {
	events, err := l.Inner.SearchEvents(fromUTC, toUTC, query)
	return events, trace.Wrap(err)
}

func (l *AuditLog) SearchSessionEvents(fromUTC, toUTC time.Time) ([]events.EventFields, error) {
	events, err := l.Inner.SearchSessionEvents(fromUTC, toUTC)
	return events, trace.Wrap(err)
}

func (l *AuditLog) WaitForDelivery(ctx context.Context) error {
	return trace.Wrap(l.Inner.WaitForDelivery(ctx))
}

func (l *AuditLog) Close() error {
	return trace.Wrap(l.Inner.Close())
}

// anonymize returns the anonymized hash of the provided data
func (l *AuditLog) anonymize(data string) string {
	h := hmac.New(sha256.New, []byte(l.AnonymizeKey))
	h.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
