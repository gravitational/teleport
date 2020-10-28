/*
Copyright 2020 Gravitational, Inc.

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
	"bytes"
	"encoding/json"
	"time"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/types"
	"github.com/gravitational/trace"
)

// EncodeMap encodes map[string]interface{} to map<string, Value>
func EncodeMap(msg map[string]interface{}) (*Struct, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pbs := types.Struct{}
	if err = jsonpb.Unmarshal(bytes.NewReader(data), &pbs); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Struct{Struct: pbs}, nil
}

// EncodeMapStrings encodes map[string][]string to map<string, Value>
func EncodeMapStrings(msg map[string][]string) (*Struct, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pbs := types.Struct{}
	if err = jsonpb.Unmarshal(bytes.NewReader(data), &pbs); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Struct{Struct: pbs}, nil
}

// MustEncodeMap panics if EncodeMap returns error
func MustEncodeMap(msg map[string]interface{}) *Struct {
	m, err := EncodeMap(msg)
	if err != nil {
		panic(err)
	}
	return m
}

// decodeToMap converts a pb.Struct to a map from strings to Go types.
func decodeToMap(s *types.Struct) (map[string]interface{}, error) {
	if s == nil {
		return nil, nil
	}
	m := map[string]interface{}{}
	for k, v := range s.Fields {
		var err error
		m[k], err = decodeValue(v)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return m, nil
}

// decodeValue decodes proto value to golang type
func decodeValue(v *types.Value) (interface{}, error) {
	switch k := v.Kind.(type) {
	case *types.Value_NullValue:
		return nil, nil
	case *types.Value_NumberValue:
		return k.NumberValue, nil
	case *types.Value_StringValue:
		return k.StringValue, nil
	case *types.Value_BoolValue:
		return k.BoolValue, nil
	case *types.Value_StructValue:
		return decodeToMap(k.StructValue)
	case *types.Value_ListValue:
		s := make([]interface{}, len(k.ListValue.Values))
		for i, e := range k.ListValue.Values {
			var err error
			s[i], err = decodeValue(e)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return s, nil
	default:
		return nil, trace.BadParameter("protostruct: unknown kind %v", k)
	}
}

// Struct is a wrapper around types.Struct
// that marshals itself into json
type Struct struct {
	types.Struct
}

// MarshalJSON marshals boolean value.
func (s *Struct) MarshalJSON() ([]byte, error) {
	m, err := decodeToMap(&s.Struct)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return utils.FastMarshal(m)
}

// UnmarshalJSON unmarshals JSON from string or bool,
// in case if value is missing or not recognized, defaults to false
func (s *Struct) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	err := jsonpb.Unmarshal(bytes.NewReader(data), &s.Struct)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetType returns event type
func (m *Metadata) GetType() string {
	return m.Type
}

// SetType sets unique type
func (m *Metadata) SetType(etype string) {
	m.Type = etype
}

// GetID returns event ID
func (m *Metadata) GetID() string {
	return m.ID
}

// GetCode returns event code
func (m *Metadata) GetCode() string {
	return m.Code
}

// SetCode sets event code
func (m *Metadata) SetCode(code string) {
	m.Code = code
}

// SetID sets event ID
func (m *Metadata) SetID(id string) {
	m.ID = id
}

// GetTime returns event time
func (m *Metadata) GetTime() time.Time {
	return m.Time
}

// SetTime sets event time
func (m *Metadata) SetTime(tm time.Time) {
	m.Time = tm
}

// SetIndex sets event index
func (m *Metadata) SetIndex(idx int64) {
	m.Index = idx
}

// GetIndex gets event index
func (m *Metadata) GetIndex() int64 {
	return m.Index
}

// GetClusterName returns originating teleport cluster name
func (m *Metadata) GetClusterName() string {
	return m.ClusterName
}

// SetClusterName returns originating teleport cluster name
func (m *Metadata) SetClusterName(clusterName string) {
	m.ClusterName = clusterName
}

// GetServerID returns event server ID
func (m *ServerMetadata) GetServerID() string {
	return m.ServerID
}

// SetServerID sets event server ID
func (m *ServerMetadata) SetServerID(id string) {
	m.ServerID = id
}

// GetServerNamespace returns event server ID
func (m *ServerMetadata) GetServerNamespace() string {
	return m.ServerNamespace
}

// SetServerNamespace sets server namespace
func (m *ServerMetadata) SetServerNamespace(ns string) {
	m.ServerNamespace = ns
}

// GetSessionID returns event session ID
func (m *SessionMetadata) GetSessionID() string {
	return m.SessionID
}

// MustToOneOf converts audit event to OneOf
// or panics, used in tests
func MustToOneOf(in AuditEvent) *OneOf {
	out, err := ToOneOf(in)
	if err != nil {
		panic(err)
	}
	return out
}

// ToOneOf converts audit event to union type of the events
func ToOneOf(in AuditEvent) (*OneOf, error) {
	out := OneOf{}

	switch e := in.(type) {
	case *UserLogin:
		out.Event = &OneOf_UserLogin{
			UserLogin: e,
		}
	case *UserCreate:
		out.Event = &OneOf_UserCreate{
			UserCreate: e,
		}
	case *UserDelete:
		out.Event = &OneOf_UserDelete{
			UserDelete: e,
		}
	case *UserPasswordChange:
		out.Event = &OneOf_UserPasswordChange{
			UserPasswordChange: e,
		}
	case *SessionStart:
		out.Event = &OneOf_SessionStart{
			SessionStart: e,
		}
	case *SessionJoin:
		out.Event = &OneOf_SessionJoin{
			SessionJoin: e,
		}
	case *SessionPrint:
		out.Event = &OneOf_SessionPrint{
			SessionPrint: e,
		}
	case *SessionReject:
		out.Event = &OneOf_SessionReject{
			SessionReject: e,
		}
	case *Resize:
		out.Event = &OneOf_Resize{
			Resize: e,
		}
	case *SessionEnd:
		out.Event = &OneOf_SessionEnd{
			SessionEnd: e,
		}
	case *SessionCommand:
		out.Event = &OneOf_SessionCommand{
			SessionCommand: e,
		}
	case *SessionDisk:
		out.Event = &OneOf_SessionDisk{
			SessionDisk: e,
		}
	case *SessionNetwork:
		out.Event = &OneOf_SessionNetwork{
			SessionNetwork: e,
		}
	case *SessionData:
		out.Event = &OneOf_SessionData{
			SessionData: e,
		}
	case *SessionLeave:
		out.Event = &OneOf_SessionLeave{
			SessionLeave: e,
		}
	case *PortForward:
		out.Event = &OneOf_PortForward{
			PortForward: e,
		}
	case *X11Forward:
		out.Event = &OneOf_X11Forward{
			X11Forward: e,
		}
	case *Subsystem:
		out.Event = &OneOf_Subsystem{
			Subsystem: e,
		}
	case *SCP:
		out.Event = &OneOf_SCP{
			SCP: e,
		}
	case *Exec:
		out.Event = &OneOf_Exec{
			Exec: e,
		}
	case *ClientDisconnect:
		out.Event = &OneOf_ClientDisconnect{
			ClientDisconnect: e,
		}
	case *AuthAttempt:
		out.Event = &OneOf_AuthAttempt{
			AuthAttempt: e,
		}
	case *AccessRequestCreate:
		out.Event = &OneOf_AccessRequestCreate{
			AccessRequestCreate: e,
		}
	case *RoleCreate:
		out.Event = &OneOf_RoleCreate{
			RoleCreate: e,
		}
	case *RoleDelete:
		out.Event = &OneOf_RoleDelete{
			RoleDelete: e,
		}
	case *ResetPasswordTokenCreate:
		out.Event = &OneOf_ResetPasswordTokenCreate{
			ResetPasswordTokenCreate: e,
		}
	case *TrustedClusterCreate:
		out.Event = &OneOf_TrustedClusterCreate{
			TrustedClusterCreate: e,
		}
	case *TrustedClusterDelete:
		out.Event = &OneOf_TrustedClusterDelete{
			TrustedClusterDelete: e,
		}
	case *TrustedClusterTokenCreate:
		out.Event = &OneOf_TrustedClusterTokenCreate{
			TrustedClusterTokenCreate: e,
		}
	case *GithubConnectorCreate:
		out.Event = &OneOf_GithubConnectorCreate{
			GithubConnectorCreate: e,
		}
	case *GithubConnectorDelete:
		out.Event = &OneOf_GithubConnectorDelete{
			GithubConnectorDelete: e,
		}
	case *OIDCConnectorCreate:
		out.Event = &OneOf_OIDCConnectorCreate{
			OIDCConnectorCreate: e,
		}
	case *OIDCConnectorDelete:
		out.Event = &OneOf_OIDCConnectorDelete{
			OIDCConnectorDelete: e,
		}
	case *SAMLConnectorCreate:
		out.Event = &OneOf_SAMLConnectorCreate{
			SAMLConnectorCreate: e,
		}
	case *SAMLConnectorDelete:
		out.Event = &OneOf_SAMLConnectorDelete{
			SAMLConnectorDelete: e,
		}
	case *KubeRequest:
		out.Event = &OneOf_KubeRequest{
			KubeRequest: e,
		}
	case *AppSessionStart:
		out.Event = &OneOf_AppSessionStart{
			AppSessionStart: e,
		}
	case *AppSessionChunk:
		out.Event = &OneOf_AppSessionChunk{
			AppSessionChunk: e,
		}
	case *AppSessionRequest:
		out.Event = &OneOf_AppSessionRequest{
			AppSessionRequest: e,
		}
	case *DatabaseSessionStart:
		out.Event = &OneOf_DatabaseSessionStart{
			DatabaseSessionStart: e,
		}
	case *DatabaseSessionEnd:
		out.Event = &OneOf_DatabaseSessionEnd{
			DatabaseSessionEnd: e,
		}
	case *DatabaseSessionQuery:
		out.Event = &OneOf_DatabaseSessionQuery{
			DatabaseSessionQuery: e,
		}
	default:
		return nil, trace.BadParameter("event type %T is not supported", in)
	}
	return &out, nil
}

// FromOneOf converts audit event from one of wrapper to interface
func FromOneOf(in OneOf) (AuditEvent, error) {
	if e := in.GetUserLogin(); e != nil {
		return e, nil
	} else if e := in.GetUserCreate(); e != nil {
		return e, nil
	} else if e := in.GetUserDelete(); e != nil {
		return e, nil
	} else if e := in.GetUserPasswordChange(); e != nil {
		return e, nil
	} else if e := in.GetSessionStart(); e != nil {
		return e, nil
	} else if e := in.GetSessionJoin(); e != nil {
		return e, nil
	} else if e := in.GetSessionPrint(); e != nil {
		return e, nil
	} else if e := in.GetSessionReject(); e != nil {
		return e, nil
	} else if e := in.GetResize(); e != nil {
		return e, nil
	} else if e := in.GetSessionEnd(); e != nil {
		return e, nil
	} else if e := in.GetSessionCommand(); e != nil {
		return e, nil
	} else if e := in.GetSessionDisk(); e != nil {
		return e, nil
	} else if e := in.GetSessionNetwork(); e != nil {
		return e, nil
	} else if e := in.GetSessionData(); e != nil {
		return e, nil
	} else if e := in.GetSessionLeave(); e != nil {
		return e, nil
	} else if e := in.GetPortForward(); e != nil {
		return e, nil
	} else if e := in.GetX11Forward(); e != nil {
		return e, nil
	} else if e := in.GetSCP(); e != nil {
		return e, nil
	} else if e := in.GetExec(); e != nil {
		return e, nil
	} else if e := in.GetSubsystem(); e != nil {
		return e, nil
	} else if e := in.GetClientDisconnect(); e != nil {
		return e, nil
	} else if e := in.GetAuthAttempt(); e != nil {
		return e, nil
	} else if e := in.GetAccessRequestCreate(); e != nil {
		return e, nil
	} else if e := in.GetResetPasswordTokenCreate(); e != nil {
		return e, nil
	} else if e := in.GetRoleCreate(); e != nil {
		return e, nil
	} else if e := in.GetRoleDelete(); e != nil {
		return e, nil
	} else if e := in.GetTrustedClusterCreate(); e != nil {
		return e, nil
	} else if e := in.GetTrustedClusterDelete(); e != nil {
		return e, nil
	} else if e := in.GetTrustedClusterTokenCreate(); e != nil {
		return e, nil
	} else if e := in.GetGithubConnectorCreate(); e != nil {
		return e, nil
	} else if e := in.GetGithubConnectorDelete(); e != nil {
		return e, nil
	} else if e := in.GetOIDCConnectorCreate(); e != nil {
		return e, nil
	} else if e := in.GetOIDCConnectorDelete(); e != nil {
		return e, nil
	} else if e := in.GetSAMLConnectorCreate(); e != nil {
		return e, nil
	} else if e := in.GetSAMLConnectorDelete(); e != nil {
		return e, nil
	} else if e := in.GetKubeRequest(); e != nil {
		return e, nil
	} else if e := in.GetAppSessionStart(); e != nil {
		return e, nil
	} else if e := in.GetAppSessionChunk(); e != nil {
		return e, nil
	} else if e := in.GetAppSessionRequest(); e != nil {
		return e, nil
	} else if e := in.GetDatabaseSessionStart(); e != nil {
		return e, nil
	} else if e := in.GetDatabaseSessionEnd(); e != nil {
		return e, nil
	} else if e := in.GetDatabaseSessionQuery(); e != nil {
		return e, nil
	} else {
		if in.Event == nil {
			return nil, trace.BadParameter("failed to parse event, session record is corrupted")
		}
		return nil, trace.BadParameter("received unsupported event %T", in.Event)
	}
}
