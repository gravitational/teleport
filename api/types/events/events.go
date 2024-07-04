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

package events

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/utils"
)

func trimN(s string, n int) string {
	// Starting at 2 to leave room for quotes at the begging and end.
	charCount := 2
	for i, r := range s {
		// Make sure we always have room to add an escape character if necessary.
		if charCount+1 > n {
			return s[:i]
		}
		if r == rune('"') || r == '\\' {
			charCount++
		}
		charCount++
	}
	return s
}

func maxSizePerField(maxLength, customFields int) int {
	if customFields == 0 {
		return maxLength
	}
	return maxLength / customFields
}

// TrimToMaxSize trims the DatabaseSessionQuery message content. The maxSize is used to calculate
// per-field max size where only user input message fields DatabaseQuery and DatabaseQueryParameters are taken into
// account.
func (m *DatabaseSessionQuery) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.DatabaseQuery = ""
	out.DatabaseQueryParameters = nil

	// Use 10% max size ballast + message size without custom fields.
	sizeBallast := maxSize/10 + out.Size()
	maxSize -= sizeBallast

	// Check how many custom fields are set.
	customFieldsCount := 0
	if m.DatabaseQuery != "" {
		customFieldsCount++
	}
	for range m.DatabaseQueryParameters {
		customFieldsCount++
	}

	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.DatabaseQuery = trimN(m.DatabaseQuery, maxFieldsSize)
	if m.DatabaseQueryParameters != nil {
		out.DatabaseQueryParameters = make([]string, len(m.DatabaseQueryParameters))
	}
	for i, v := range m.DatabaseQueryParameters {
		out.DatabaseQueryParameters[i] = trimN(v, maxFieldsSize)
	}
	return out
}

// TrimToMaxSize trims the SessionStart event to the given maximum size.
// Currently assumes that the largest field will be InitialCommand and tries to
// trim that.
func (e *SessionStart) TrimToMaxSize(maxSize int) AuditEvent {
	size := e.Size()
	if size <= maxSize {
		return e
	}

	out := utils.CloneProtoMsg(e)
	out.InitialCommand = nil

	// Use 10% max size ballast + message size without InitialCommand
	sizeBallast := maxSize/10 + out.Size()
	maxSize -= sizeBallast

	maxFieldSize := maxSizePerField(maxSize, len(e.InitialCommand))

	out.InitialCommand = make([]string, len(e.InitialCommand))
	for i, c := range e.InitialCommand {
		out.InitialCommand[i] = trimN(c, maxFieldSize)
	}

	return out
}

// TrimToMaxSize trims the Exec event to the given maximum size.
// Currently assumes that the largest field will be Command and tries to trim
// that.
func (e *Exec) TrimToMaxSize(maxSize int) AuditEvent {
	size := e.Size()
	if size <= maxSize {
		return e
	}

	out := utils.CloneProtoMsg(e)
	out.Command = ""

	// Use 10% max size ballast + message size without Command
	sizeBallast := maxSize/10 + out.Size()
	maxSize -= sizeBallast

	out.Command = trimN(e.Command, maxSize)

	return out
}

// TrimToMaxSize trims the UserLogin event to the given maximum size.
// The initial implementation is to cover concerns that a malicious user could
// craft a request that creates error messages too large to be handled by the
// underlying storage and thus cause the events to be omitted entirely. See
// teleport-private#172.
func (e *UserLogin) TrimToMaxSize(maxSize int) AuditEvent {
	size := e.Size()
	if size <= maxSize {
		return e
	}

	out := utils.CloneProtoMsg(e)
	out.Status.Error = ""
	out.Status.UserMessage = ""

	// Use 10% max size ballast + message size without Error and UserMessage
	sizeBallast := maxSize/10 + out.Size()
	maxSize -= sizeBallast

	maxFieldSize := maxSizePerField(maxSize, 2)

	out.Status.Error = trimN(e.Status.Error, maxFieldSize)
	out.Status.UserMessage = trimN(e.Status.UserMessage, maxFieldSize)

	return out
}

// ToUnstructured converts the event stored in the AuditEvent interface
// to unstructured.
// If the event is a session print event, it is converted to a plugins printEvent struct
// which is then converted to structpb.Struct. Otherwise the event is marshaled directly.
func ToUnstructured(evt AuditEvent) (*auditlogpb.EventUnstructured, error) {
	payload, err := json.Marshal(evt)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	id := computeEventID(evt, payload)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	str := &structpb.Struct{}
	if err := str.UnmarshalJSON(payload); err != nil {
		return nil, trace.Wrap(err)
	}

	// If the event is a session print event, convert it to a printEvent struct
	// to include the `data` field in the JSON.
	if p, ok := evt.(*SessionPrint); ok {
		const printEventDataKey = "data"
		// append the `data` field to the unstructured event
		str.Fields[printEventDataKey], err = structpb.NewValue(p.Data)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &auditlogpb.EventUnstructured{
		Type:         evt.GetType(),
		Index:        evt.GetIndex(),
		Time:         timestamppb.New(evt.GetTime()),
		Id:           id,
		Unstructured: str,
	}, nil
}

// computeEventID computes the ID of the event. If the event already has an ID, it is returned.
// Otherwise, the event is marshaled to JSON and the SHA256 hash of the JSON is returned.
func computeEventID(evt AuditEvent, payload []byte) string {
	id := evt.GetID()
	if id != "" {
		return id
	}

	hash := sha256.Sum256(payload)
	return hex.EncodeToString(hash[:])
}
