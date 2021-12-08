/*
Copyright 2018 Gravitational, Inc.

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

package events

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
)

// DiscardAuditLog is do-nothing, discard-everything implementation
// of IAuditLog interface used for cases when audit is turned off
type DiscardAuditLog struct{}

// NewDiscardAuditLog returns a no-op audit log instance
func NewDiscardAuditLog() *DiscardAuditLog {
	return &DiscardAuditLog{}
}

func (d *DiscardAuditLog) WaitForDelivery(context.Context) error {
	return nil
}

func (d *DiscardAuditLog) Close() error {
	return nil
}
func (d *DiscardAuditLog) EmitAuditEventLegacy(event Event, fields EventFields) error {
	return nil
}
func (d *DiscardAuditLog) PostSessionSlice(SessionSlice) error {
	return nil
}
func (d *DiscardAuditLog) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	return make([]byte, 0), nil
}
func (d *DiscardAuditLog) GetSessionEvents(namespace string, sid session.ID, after int, includePrintEvents bool) ([]EventFields, error) {
	return make([]EventFields, 0), nil
}
func (d *DiscardAuditLog) SearchEvents(fromUTC, toUTC time.Time, namespace string, eventType []string, limit int, order types.EventOrder, startKey string) ([]apievents.AuditEvent, string, error) {
	return make([]apievents.AuditEvent, 0), "", nil
}
func (d *DiscardAuditLog) SearchSessionEvents(fromUTC, toUTC time.Time, limit int, order types.EventOrder, startKey string, cond *types.WhereExpr) ([]apievents.AuditEvent, string, error) {
	return make([]apievents.AuditEvent, 0), "", nil
}
func (d *DiscardAuditLog) UploadSessionRecording(SessionRecording) error {
	return nil
}
func (d *DiscardAuditLog) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	return nil
}
func (d *DiscardAuditLog) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	c, e := make(chan apievents.AuditEvent), make(chan error, 1)
	close(c)
	return c, e
}
