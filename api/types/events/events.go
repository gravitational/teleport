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

	"github.com/gogo/protobuf/types"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils"
)

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

// trimStr trims a string to a given length.
func trimStr(s string, n int) string {
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

// maxSizePerField returns the maximum size each field can be when trimming.
func maxSizePerField(maxSize, customFields int) int {
	if customFields == 0 {
		return maxSize
	}
	return maxSize / customFields
}

// adjustedMaxSize returns the maximum size to trim an event to after
// accounting for the size of the message without custom fields that
// will be trimmed.
func adjustedMaxSize(e AuditEvent, maxSize int) int {
	// Use 10% max size ballast + message size without custom fields.
	sizeBallast := maxSize/10 + e.Size()
	return maxSize - sizeBallast
}

// nonEmptyStrs returns the number of non-empty strings.
func nonEmptyStrs(s ...string) int {
	nonEmptyStrs := 0
	for _, s := range s {
		if s != "" {
			nonEmptyStrs++
		}
	}
	return nonEmptyStrs
}

// nonEmptyStrsInSlice returns the number of non-empty elements in a
// slice of strings.
func nonEmptyStrsInSlice[T ~string](s ...[]T) int {
	nonEmptyStrs := 0
	for _, s := range s {
		if len(s) != 0 {
			nonEmptyStrs += len(s)
		}
	}
	return nonEmptyStrs
}

// trimStrSlice trims each element in a slice of strings to a given
// length.
func trimStrSlice[T ~string](strs []T, maxSize int) []T {
	if len(strs) == 0 {
		return nil
	}
	trimmed := make([]T, len(strs))
	for i, v := range strs {
		trimmed[i] = T(trimStr(string(v), maxSize))
	}
	return trimmed
}

func (m *Status) nonEmptyStrs() int {
	return nonEmptyStrs(m.Error, m.UserMessage)
}

func (m *Status) trimToMaxSize(maxSize int) Status {
	var out Status
	out.Error = trimStr(m.Error, maxSize)
	out.UserMessage = trimStr(m.UserMessage, maxSize)
	return out
}

func (m *Struct) nonEmptyStrs() int {
	var toTrim int
	for _, v := range m.Fields {
		toTrim++

		if v != nil {
			if v.GetStringValue() != "" {
				toTrim++
				continue
			}
			if l := v.GetListValue(); l != nil {
				for _, lv := range l.Values {
					if lv.GetStringValue() != "" {
						toTrim++
					}
				}
			}
		}
	}

	return toTrim
}

func (m *Struct) trimToMaxSize(maxSize int) *Struct {
	var out Struct
	for k, v := range m.Fields {
		delete(out.Fields, k)
		trimmedKey := trimStr(k, maxSize)

		if v != nil {
			if strVal := v.GetStringValue(); strVal != "" {
				trimmedVal := trimStr(strVal, maxSize)
				out.Fields[trimmedKey] = &types.Value{
					Kind: &types.Value_StringValue{
						StringValue: trimmedVal,
					},
				}
			} else if l := v.GetListValue(); l != nil {
				for i, lv := range l.Values {
					if strVal := lv.GetStringValue(); strVal != "" {
						trimmedVal := trimStr(strVal, maxSize)
						l.Values[i] = &types.Value{
							Kind: &types.Value_StringValue{
								StringValue: trimmedVal,
							},
						}
					}
				}
			}
		}
	}
	return &out
}

func (m *CommandMetadata) nonEmptyStrs() int {
	return nonEmptyStrs(m.Command, m.Error)
}

func (m *CommandMetadata) trimToMaxSize(maxSize int) CommandMetadata {
	var out CommandMetadata
	out.Command = trimStr(m.Command, maxSize)
	out.Error = trimStr(m.Error, maxSize)
	return out
}

// nonEmptyStrsInMap returns the number of non-empty keys and values in
// a map of strings to strings.
func nonEmptyStrsInMap(m map[string]string) int {
	var toTrim int
	for k, v := range m {
		if k != "" {
			toTrim++
		}
		if v != "" {
			toTrim++
		}
	}
	return toTrim
}

// trimMap trims each key and value in a map of strings to strings
// to a given length.
func trimMap(m map[string]string, maxSize int) map[string]string {
	for k, v := range m {
		delete(m, k)
		trimmedKey := trimStr(k, maxSize)
		m[trimmedKey] = trimStr(v, maxSize)
	}
	return m
}

// nonEmptyTraits returns the number of non-empty keys and values in
// some traits.
func nonEmptyTraits(traits wrappers.Traits) int {
	var toTrim int
	for k, vals := range traits {
		if k != "" {
			toTrim++
		}
		for _, v := range vals {
			if v != "" {
				toTrim++
			}
		}
	}
	return toTrim
}

// trimTraits trims each key and value in some traits to a given length.
func trimTraits(traits wrappers.Traits, maxSize int) wrappers.Traits {
	for k, vals := range traits {
		delete(traits, k)
		trimmedKey := trimStr(k, maxSize)
		traits[trimmedKey] = trimStrSlice(vals, maxSize)
	}
	return traits
}

// TrimToMaxSize returns the SessionPrint event unmodified because there
// are no string fields to trim.
func (m *SessionPrint) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

// TrimToMaxSize trims the SessionStart event to the given maximum size.
// Currently assumes that the largest field will be InitialCommand and tries to
// trim that.
func (m *SessionStart) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.InitialCommand = nil
	out.Invited = nil

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrsInSlice(m.InitialCommand)
	maxFieldSize := maxSizePerField(maxSize, customFieldsCount)

	out.InitialCommand = trimStrSlice(m.InitialCommand, maxFieldSize)

	customFieldsCount = nonEmptyStrsInSlice(m.Invited)
	maxFieldSize = maxSizePerField(maxSize, customFieldsCount)
	out.Invited = trimStrSlice(m.Invited, maxFieldSize)

	return out
}

func (m *SessionEnd) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.InitialCommand = nil

	maxSize = adjustedMaxSize(out, maxSize)

	// Check how many custom fields are set.
	customFieldsCount := nonEmptyStrsInSlice(m.InitialCommand)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.InitialCommand = trimStrSlice(m.InitialCommand, maxFieldsSize)

	return out
}

func (m *SessionUpload) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *SessionJoin) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *SessionLeave) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *SessionData) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *ClientDisconnect) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Reason = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Reason)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Reason = trimStr(m.Reason, maxFieldsSize)

	return out
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

	maxSize = adjustedMaxSize(out, maxSize)

	// Check how many custom fields are set.
	customFieldsCount := nonEmptyStrs(m.DatabaseQuery) + nonEmptyStrsInSlice(m.DatabaseQueryParameters)

	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.DatabaseQuery = trimStr(m.DatabaseQuery, maxFieldsSize)
	out.DatabaseQueryParameters = trimStrSlice(m.DatabaseQueryParameters, maxFieldsSize)

	return out
}

// TrimToMaxSize trims the Exec event to the given maximum size.
// Currently assumes that the largest field will be Command and tries to trim
// that.
func (m *Exec) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.CommandMetadata = CommandMetadata{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.CommandMetadata.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.CommandMetadata = m.CommandMetadata.trimToMaxSize(maxFieldsSize)

	return out
}

// TrimToMaxSize trims the UserLogin event to the given maximum size.
// The initial implementation is to cover concerns that a malicious user could
// craft a request that creates error messages too large to be handled by the
// underlying storage and thus cause the events to be omitted entirely. See
// teleport-private#172.
func (m *UserLogin) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)
	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldSize)

	return out
}

func (m *UserDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *UserCreate) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Roles = nil

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrsInSlice(m.Roles)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Roles = trimStrSlice(m.Roles, maxFieldsSize)

	return out
}

func (m *UserUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Roles = nil

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrsInSlice(m.Roles)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Roles = trimStrSlice(m.Roles, maxFieldsSize)

	return out
}

func (m *UserPasswordChange) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *AccessRequestCreate) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Roles = nil
	out.Reason = ""
	out.Annotations = nil

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrsInSlice(m.Roles) +
		nonEmptyStrs(m.Reason) +
		m.Annotations.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Roles = trimStrSlice(m.Roles, maxFieldsSize)
	out.Reason = trimStr(m.Reason, maxFieldsSize)
	out.Annotations = m.Annotations.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *AccessRequestResourceSearch) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.SearchAsRoles = nil
	out.Labels = nil
	out.PredicateExpression = ""
	out.SearchKeywords = nil

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrsInSlice(m.SearchAsRoles, m.SearchKeywords) +
		nonEmptyStrsInMap(m.Labels) +
		nonEmptyStrs(m.PredicateExpression)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.SearchAsRoles = trimStrSlice(m.SearchAsRoles, maxFieldsSize)
	out.Labels = trimMap(m.Labels, maxFieldsSize)
	out.PredicateExpression = trimStr(m.PredicateExpression, maxFieldsSize)
	out.SearchKeywords = trimStrSlice(m.SearchKeywords, maxFieldsSize)

	return out
}

func (m *BillingCardCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *BillingCardDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *BillingInformationUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *UserTokenCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *Subsystem) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Name = ""
	out.Error = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Name, m.Error)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Name = trimStr(m.Name, maxFieldsSize)
	out.Error = trimStr(m.Error, maxFieldsSize)

	return out
}

func (m *X11Forward) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *PortForward) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *AuthAttempt) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *SCP) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Path = ""
	out.CommandMetadata = CommandMetadata{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Path) + m.CommandMetadata.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Path = trimStr(m.Path, maxFieldsSize)
	out.CommandMetadata = m.CommandMetadata.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *Resize) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *SessionCommand) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Path = ""
	out.Argv = nil

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Path) + nonEmptyStrsInSlice(m.Argv)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Path = trimStr(m.Path, maxFieldsSize)
	out.Argv = trimStrSlice(m.Argv, maxFieldsSize)

	return out
}

func (m *SessionDisk) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Path = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Path)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Path = trimStr(m.Path, maxFieldsSize)

	return out
}

func (m *SessionNetwork) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *RoleCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *RoleUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *RoleDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *TrustedClusterCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *TrustedClusterDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *TrustedClusterTokenCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *ProvisionTokenCreate) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Roles = nil

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrsInSlice(m.Roles)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Roles = trimStrSlice(m.Roles, maxFieldsSize)

	return out
}

func (m *GithubConnectorCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *GithubConnectorUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *GithubConnectorDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *OIDCConnectorCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *OIDCConnectorUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *OIDCConnectorDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *SAMLConnectorCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *SAMLConnectorUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *SAMLConnectorDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *SessionReject) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Reason = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Reason)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Reason = trimStr(m.Reason, maxFieldsSize)

	return out
}

func (m *AppSessionStart) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *AppSessionEnd) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *AppSessionChunk) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *AppSessionRequest) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Path = ""
	out.RawQuery = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Path, m.RawQuery)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Path = trimStr(m.Path, maxFieldsSize)
	out.RawQuery = trimStr(m.RawQuery, maxFieldsSize)

	return out
}

func (m *AppSessionDynamoDBRequest) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Path = ""
	out.RawQuery = ""
	out.Target = ""
	out.Body = nil

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Path, m.RawQuery, m.Target) + m.Body.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Path = trimStr(m.Path, maxFieldsSize)
	out.RawQuery = trimStr(m.RawQuery, maxFieldsSize)
	out.Target = trimStr(m.Target, maxFieldsSize)
	out.Body = m.Body.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *AppCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *AppUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *AppDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *DatabaseCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *DatabaseUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *DatabaseDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *DatabaseSessionStart) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *DatabaseSessionEnd) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *DatabaseSessionMalformedPacket) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Payload = nil

	maxSize = adjustedMaxSize(out, maxSize)

	var customFieldsCount int
	if len(m.Payload) != 0 {
		customFieldsCount++
	}
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Payload = []byte(trimStr(string(m.Payload), maxFieldsSize))

	return out
}

func (m *DatabasePermissionUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *DatabaseUserCreate) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}
	out.Username = ""
	out.Roles = nil

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs() + nonEmptyStrs(m.Username) + nonEmptyStrsInSlice(m.Roles)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)
	out.Username = trimStr(m.Username, maxFieldsSize)
	out.Roles = trimStrSlice(m.Roles, maxFieldsSize)

	return out
}

func (m *DatabaseUserDeactivate) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}
	out.Username = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs() + nonEmptyStrs(m.Username)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)
	out.Username = trimStr(m.Username, maxFieldsSize)

	return out
}

func (m *PostgresParse) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.StatementName = ""
	out.Query = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.StatementName, m.Query)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.StatementName = trimStr(m.StatementName, maxFieldsSize)
	out.Query = trimStr(m.Query, maxFieldsSize)

	return out
}

func (m *PostgresBind) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.StatementName = ""
	out.PortalName = ""
	out.Parameters = nil

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.StatementName, m.PortalName) + nonEmptyStrsInSlice(m.Parameters)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.StatementName = trimStr(m.StatementName, maxFieldsSize)
	out.PortalName = trimStr(m.PortalName, maxFieldsSize)
	out.Parameters = trimStrSlice(m.Parameters, maxFieldsSize)

	return out
}

func (m *PostgresExecute) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.PortalName = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.PortalName)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.PortalName = trimStr(m.PortalName, maxFieldsSize)

	return out
}

func (m *PostgresClose) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.StatementName = ""
	out.PortalName = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.StatementName, m.PortalName)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.StatementName = trimStr(m.StatementName, maxFieldsSize)
	out.PortalName = trimStr(m.PortalName, maxFieldsSize)

	return out
}

func (m *PostgresFunctionCall) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.FunctionArgs = nil

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrsInSlice(m.FunctionArgs)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.FunctionArgs = trimStrSlice(m.FunctionArgs, maxFieldsSize)

	return out
}

func (m *MySQLStatementPrepare) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Query = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Query)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Query = trimStr(m.Query, maxFieldsSize)

	return out
}

func (m *MySQLStatementExecute) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Parameters = nil

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrsInSlice(m.Parameters)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Parameters = trimStrSlice(m.Parameters, maxFieldsSize)

	return out
}

func (m *MySQLStatementSendLongData) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *MySQLStatementClose) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *MySQLStatementReset) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *MySQLStatementFetch) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *MySQLStatementBulkExecute) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Parameters = nil

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrsInSlice(m.Parameters)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Parameters = trimStrSlice(m.Parameters, maxFieldsSize)

	return out
}

func (m *MySQLInitDB) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.SchemaName = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.SchemaName)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.SchemaName = trimStr(m.SchemaName, maxFieldsSize)

	return out
}

func (m *MySQLCreateDB) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.SchemaName = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.SchemaName)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.SchemaName = trimStr(m.SchemaName, maxFieldsSize)

	return out
}

func (m *MySQLDropDB) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.SchemaName = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.SchemaName)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.SchemaName = trimStr(m.SchemaName, maxFieldsSize)

	return out
}

func (m *MySQLShutDown) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *MySQLProcessKill) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *MySQLDebug) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *MySQLRefresh) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Subcommand = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Subcommand)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Subcommand = trimStr(m.Subcommand, maxFieldsSize)

	return out
}

func (m *SQLServerRPCRequest) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Procname = ""
	out.Parameters = nil

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Procname) + nonEmptyStrsInSlice(m.Parameters)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Procname = trimStr(m.Procname, maxFieldsSize)
	out.Parameters = trimStrSlice(m.Parameters, maxFieldsSize)

	return out
}

func (m *ElasticsearchRequest) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Path = ""
	out.RawQuery = ""
	out.Body = nil
	out.Headers = nil
	out.Target = ""
	out.Query = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Path, m.RawQuery, m.Target, m.Query) + nonEmptyTraits(m.Headers)
	if len(m.Body) != 0 {
		customFieldsCount++
	}
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Path = trimStr(m.Path, maxFieldsSize)
	out.RawQuery = trimStr(m.RawQuery, maxFieldsSize)
	out.Body = []byte(trimStr(string(m.Body), maxFieldsSize))
	out.Headers = trimTraits(m.Headers, maxFieldsSize)
	out.Target = trimStr(m.Target, maxFieldsSize)
	out.Query = trimStr(m.Query, maxFieldsSize)

	return out
}

func (m *OpenSearchRequest) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Path = ""
	out.RawQuery = ""
	out.Body = nil
	out.Headers = nil
	out.Target = ""
	out.Query = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Path, m.RawQuery, m.Target, m.Query) + nonEmptyTraits(m.Headers)
	if len(m.Body) != 0 {
		customFieldsCount++
	}
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Path = trimStr(m.Path, maxFieldsSize)
	out.RawQuery = trimStr(m.RawQuery, maxFieldsSize)
	out.Body = []byte(trimStr(string(m.Body), maxFieldsSize))
	out.Headers = trimTraits(m.Headers, maxFieldsSize)
	out.Target = trimStr(m.Target, maxFieldsSize)
	out.Query = trimStr(m.Query, maxFieldsSize)

	return out
}

func (m *DynamoDBRequest) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Path = ""
	out.RawQuery = ""
	out.Body = nil
	out.Target = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Path, m.RawQuery, m.Target) + m.Body.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Path = trimStr(m.Path, maxFieldsSize)
	out.RawQuery = trimStr(m.RawQuery, maxFieldsSize)
	out.Body = m.Body.trimToMaxSize(maxFieldsSize)
	out.Target = trimStr(m.Target, maxFieldsSize)

	return out
}

func (m *KubeRequest) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.RequestPath = ""
	out.ResourceAPIGroup = ""
	out.ResourceNamespace = ""
	out.ResourceName = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.RequestPath, m.ResourceAPIGroup, m.ResourceNamespace, m.ResourceName)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.RequestPath = trimStr(m.RequestPath, maxFieldsSize)
	out.ResourceAPIGroup = trimStr(m.ResourceAPIGroup, maxFieldsSize)
	out.ResourceNamespace = trimStr(m.ResourceNamespace, maxFieldsSize)
	out.ResourceName = trimStr(m.ResourceName, maxFieldsSize)

	return out
}

func (m *MFADeviceAdd) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *MFADeviceDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *DeviceEvent) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = &Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	status := m.Status.trimToMaxSize(maxFieldsSize)
	out.Status = &status

	return out
}

func (m *DeviceEvent2) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *LockCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *LockDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *RecoveryCodeGenerate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *RecoveryCodeUsed) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *WindowsDesktopSessionStart) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}
	out.Domain = ""
	out.WindowsUser = ""
	out.DesktopLabels = nil
	out.DesktopName = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs() +
		nonEmptyStrs(m.Domain, m.WindowsUser, m.DesktopName) +
		nonEmptyStrsInMap(m.DesktopLabels)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)
	out.Domain = trimStr(m.Domain, maxFieldsSize)
	out.WindowsUser = trimStr(m.WindowsUser, maxFieldsSize)
	out.DesktopLabels = trimMap(m.DesktopLabels, maxFieldsSize)
	out.DesktopName = trimStr(m.DesktopName, maxFieldsSize)

	return out
}

func (m *WindowsDesktopSessionEnd) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Domain = ""
	out.WindowsUser = ""
	out.DesktopLabels = nil
	out.DesktopName = ""
	out.Participants = nil

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Domain, m.WindowsUser, m.DesktopName) +
		nonEmptyStrsInMap(m.DesktopLabels) +
		nonEmptyStrsInSlice(m.Participants)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Domain = trimStr(m.Domain, maxFieldsSize)
	out.WindowsUser = trimStr(m.WindowsUser, maxFieldsSize)
	out.DesktopLabels = trimMap(m.DesktopLabels, maxFieldsSize)
	out.DesktopName = trimStr(m.DesktopName, maxFieldsSize)
	out.Participants = trimStrSlice(m.Participants, maxFieldsSize)

	return out
}

func (m *DesktopClipboardSend) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *DesktopClipboardReceive) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *SessionConnect) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *AccessRequestDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *CertificateCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *DesktopRecording) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Message = nil

	maxSize = adjustedMaxSize(out, maxSize)

	var customFieldsCount int
	if len(m.Message) != 0 {
		customFieldsCount++
	}
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Message = []byte(trimStr(string(m.Message), maxFieldsSize))

	return out
}

func (m *RenewableCertificateGenerationMismatch) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *SFTP) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.WorkingDirectory = ""
	out.Path = ""
	out.TargetPath = ""
	out.Error = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.WorkingDirectory, m.Path, m.TargetPath, m.Error)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.WorkingDirectory = trimStr(m.WorkingDirectory, maxFieldsSize)
	out.Path = trimStr(m.Path, maxFieldsSize)
	out.TargetPath = trimStr(m.TargetPath, maxFieldsSize)
	out.Error = trimStr(m.Error, maxFieldsSize)

	return out
}

func (m *UpgradeWindowStartUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *SessionRecordingAccess) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *SSMRun) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = ""
	out.StandardOutput = ""
	out.StandardError = ""
	out.InvocationURL = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Status, m.StandardOutput, m.StandardError, m.InvocationURL)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = trimStr(m.Status, maxFieldsSize)
	out.StandardOutput = trimStr(m.StandardOutput, maxFieldsSize)
	out.StandardError = trimStr(m.StandardError, maxFieldsSize)
	out.InvocationURL = trimStr(m.InvocationURL, maxFieldsSize)

	return out
}

func (m *KubernetesClusterCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *KubernetesClusterUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *KubernetesClusterDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *DesktopSharedDirectoryStart) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.DirectoryName = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.DirectoryName)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.DirectoryName = trimStr(m.DirectoryName, maxFieldsSize)

	return out
}

func (m *DesktopSharedDirectoryRead) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.DirectoryName = ""
	out.Path = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.DirectoryName, m.Path)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.DirectoryName = trimStr(m.DirectoryName, maxFieldsSize)
	out.Path = trimStr(m.Path, maxFieldsSize)

	return out
}

func (m *DesktopSharedDirectoryWrite) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.DirectoryName = ""
	out.Path = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.DirectoryName, m.Path)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.DirectoryName = trimStr(m.DirectoryName, maxFieldsSize)
	out.Path = trimStr(m.Path, maxFieldsSize)

	return out
}

func (m *BotJoin) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Attributes = nil

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Attributes.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Attributes = m.Attributes.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *InstanceJoin) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *BotCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *BotUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *BotDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *LoginRuleCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *LoginRuleDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *SAMLIdPAuthAttempt) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *SAMLIdPServiceProviderCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *SAMLIdPServiceProviderUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *SAMLIdPServiceProviderDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *SAMLIdPServiceProviderDeleteAll) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *OktaResourcesUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *OktaSyncFailure) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *OktaAssignmentResult) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *OktaUserSync) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *OktaAccessListSync) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *AccessPathChanged) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *AccessListCreate) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *AccessListUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *AccessListDelete) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *AccessListReview) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *AccessListMemberCreate) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *AccessListMemberUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *AccessListMemberDelete) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *AccessListMemberDeleteAllForAccessList) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *UserLoginAccessListInvalid) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *AuditQueryRun) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}
	out.Name = ""
	out.Query = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs() + nonEmptyStrs(m.Name, m.Query)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)
	out.Name = trimStr(m.Name, maxFieldsSize)
	out.Query = trimStr(m.Query, maxFieldsSize)

	return out
}

func (m *SecurityReportRun) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *ExternalAuditStorageEnable) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *ExternalAuditStorageDisable) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *CreateMFAAuthChallenge) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *ValidateMFAAuthResponse) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *SPIFFESVIDIssued) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.SPIFFEID = ""
	out.DNSSANs = nil
	out.IPSANs = nil
	out.SVIDType = ""
	out.Hint = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.SPIFFEID, m.SVIDType, m.Hint) + nonEmptyStrsInSlice(m.DNSSANs, m.IPSANs)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.SPIFFEID = trimStr(m.SPIFFEID, maxFieldsSize)
	out.DNSSANs = trimStrSlice(m.DNSSANs, maxFieldsSize)
	out.IPSANs = trimStrSlice(m.IPSANs, maxFieldsSize)
	out.SVIDType = trimStr(m.SVIDType, maxFieldsSize)
	out.Hint = trimStr(m.Hint, maxFieldsSize)

	return out
}

func (m *AuthPreferenceUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *ClusterNetworkingConfigUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *SessionRecordingConfigUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *AccessGraphSettingsUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *SpannerRPC) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Status = Status{}
	out.Procedure = ""
	out.Args = nil

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.Status.nonEmptyStrs() + nonEmptyStrs(m.Procedure) + m.Args.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Status = m.Status.trimToMaxSize(maxFieldsSize)
	out.Procedure = trimStr(m.Procedure, maxFieldsSize)
	out.Args = m.Args.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *DatabaseSessionCommandResult) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *Unknown) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Data = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Data)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Data = trimStr(m.Data, maxFieldsSize)

	return out
}

func (m *CassandraBatch) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Keyspace = ""
	out.BatchType = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Keyspace, m.BatchType)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Keyspace = trimStr(m.Keyspace, maxFieldsSize)
	out.BatchType = trimStr(m.BatchType, maxFieldsSize)

	return out
}

func (m *CassandraRegister) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.EventTypes = nil

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrsInSlice(m.EventTypes)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.EventTypes = trimStrSlice(m.EventTypes, maxFieldsSize)

	return out
}

func (m *CassandraPrepare) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Query = ""
	out.Keyspace = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Query, m.Keyspace)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Query = trimStr(m.Query, maxFieldsSize)
	out.Keyspace = trimStr(m.Keyspace, maxFieldsSize)

	return out
}

func (m *CassandraExecute) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *DiscoveryConfigCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *DiscoveryConfigUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *DiscoveryConfigDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *DiscoveryConfigDeleteAll) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *IntegrationCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *IntegrationUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *IntegrationDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *SPIFFEFederationCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *SPIFFEFederationDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *PluginCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *PluginUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *PluginDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *StaticHostUserCreate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *StaticHostUserUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *StaticHostUserDelete) TrimToMaxSize(maxSize int) AuditEvent {
	return m
}

func (m *CrownJewelCreate) TrimToMaxSize(_ int) AuditEvent {
	return m
}

func (m *CrownJewelUpdate) TrimToMaxSize(_ int) AuditEvent {
	return m
}

func (m *CrownJewelDelete) TrimToMaxSize(_ int) AuditEvent {
	return m
}

func (m *UserTaskCreate) TrimToMaxSize(_ int) AuditEvent {
	return m
}

func (m *UserTaskUpdate) TrimToMaxSize(_ int) AuditEvent {
	return m
}

func (m *UserTaskDelete) TrimToMaxSize(_ int) AuditEvent {
	return m
}

func (m *AutoUpdateConfigCreate) TrimToMaxSize(_ int) AuditEvent {
	return m
}

func (m *AutoUpdateConfigUpdate) TrimToMaxSize(_ int) AuditEvent {
	return m
}

func (m *AutoUpdateConfigDelete) TrimToMaxSize(_ int) AuditEvent {
	return m
}

func (m *AutoUpdateVersionCreate) TrimToMaxSize(_ int) AuditEvent {
	return m
}

func (m *AutoUpdateVersionUpdate) TrimToMaxSize(_ int) AuditEvent {
	return m
}

func (m *AutoUpdateVersionDelete) TrimToMaxSize(_ int) AuditEvent {
	return m
}

func (m *WorkloadIdentityCreate) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.WorkloadIdentityData = nil

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.WorkloadIdentityData.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.WorkloadIdentityData = m.WorkloadIdentityData.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *WorkloadIdentityUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.WorkloadIdentityData = nil

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := m.WorkloadIdentityData.nonEmptyStrs()
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.WorkloadIdentityData = m.WorkloadIdentityData.trimToMaxSize(maxFieldsSize)

	return out
}

func (m *WorkloadIdentityDelete) TrimToMaxSize(_ int) AuditEvent {
	return m
}

func (m *WorkloadIdentityX509RevocationCreate) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Reason = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Reason)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Reason = trimStr(m.Reason, maxFieldsSize)

	return m
}

func (m *WorkloadIdentityX509RevocationUpdate) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := utils.CloneProtoMsg(m)
	out.Reason = ""

	maxSize = adjustedMaxSize(out, maxSize)

	customFieldsCount := nonEmptyStrs(m.Reason)
	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.Reason = trimStr(m.Reason, maxFieldsSize)

	return m
}

func (m *WorkloadIdentityX509RevocationDelete) TrimToMaxSize(_ int) AuditEvent {
	return m
}

func (m *ContactCreate) TrimToMaxSize(_ int) AuditEvent {
	return m
}

func (m *ContactDelete) TrimToMaxSize(_ int) AuditEvent {
	return m
}
