/*
Copyright 2021 Gravitational, Inc.

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

import "time"

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

// GetUser returns event teleport user
func (m *UserMetadata) GetUser() string {
	return m.User
}
