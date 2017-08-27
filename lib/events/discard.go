package events

import (
	"io"
	"time"

	"github.com/gravitational/teleport/lib/session"
)

// DiscardAuditLog is do-nothing, discard-everything implementation
// of IAuditLog interface used for cases when audit is turned off
type DiscardAuditLog struct {
}

func (d *DiscardAuditLog) Close() error {
	return nil
}

func (d *DiscardAuditLog) EmitAuditEvent(eventType string, fields EventFields) error {
	return nil
}
func (d *DiscardAuditLog) PostSessionChunk(namespace string, sid session.ID, reader io.Reader) error {
	return nil
}
func (d *DiscardAuditLog) PostSessionSlice(SessionSlice) error {
	return nil
}
func (d *DiscardAuditLog) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	return make([]byte, 0), nil
}
func (d *DiscardAuditLog) GetSessionEvents(namespace string, sid session.ID, after int) ([]EventFields, error) {
	return make([]EventFields, 0), nil
}
func (d *DiscardAuditLog) SearchEvents(fromUTC, toUTC time.Time, query string) ([]EventFields, error) {
	return make([]EventFields, 0), nil
}
func (d *DiscardAuditLog) SearchSessionEvents(fromUTC time.Time, toUTC time.Time) ([]EventFields, error) {
	return make([]EventFields, 0), nil
}
