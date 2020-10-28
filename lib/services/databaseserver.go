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

package services

import (
	"fmt"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// DatabaseServer represents a database access server.
type DatabaseServer interface {
	// Resource provides common resource methods.
	Resource
	// GetNamespace returns server namespace.
	GetNamespace() string
	// GetTeleportVersion returns the teleport version the server is running on.
	GetTeleportVersion() string
	// GetHostname returns the server hostname.
	GetHostname() string
	// GetHostID returns ID of the host the server is running on.
	GetHostID() string
	// GetStaticLabels returns server static labels.
	GetStaticLabels() map[string]string
	// SetStaticLabels sets server static labels.
	SetStaticLabels(map[string]string)
	// GetDynamicLabels returns server dynamic labels.
	GetDynamicLabels() map[string]CommandLabel
	// SetDynamicLabels sets server dynamic labels.
	SetDynamicLabels(map[string]CommandLabel)
	// GetAllLabels returns combined static and dynamic labels.
	GetAllLabels() map[string]string
	// LabelsString returns all labels as a string.
	LabelsString() string
	// GetRotation gets the state of certificate authority rotation.
	GetRotation() Rotation
	// SetRotation sets the state of certificate authority rotation.
	SetRotation(Rotation)
	// String returns string representation of the server.
	String() string
	// GetDescription returns the database server description.
	GetDescription() string
	// GetProtocol returns the database server protocol.
	GetProtocol() string
	// GetURI returns the database connection address.
	GetURI() string
	// GetCA returns the database CA certificate bytes.
	GetCA() []byte
	// GetRegion returns the AWS region for RDS/Aurora databases.
	GetRegion() string
	// IsAWS returns true if this an RDS/Aurora database.
	IsAWS() bool
	// CheckAndSetDefaults checks and set default values for any missing fields.
	CheckAndSetDefaults() error
	// Copy returns a copy of this database server object.
	Copy() DatabaseServer
}

// NewDatabaseServerV2 creates a new database server instance.
func NewDatabaseServerV2(name string, labels map[string]string, spec DatabaseServerSpecV2) *DatabaseServerV2 {
	return &DatabaseServerV2{
		Kind:    KindDatabaseServer,
		Version: V2,
		Metadata: Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
			Labels:    labels,
		},
		Spec: spec,
	}
}

// GetVersion returns the database server resource version.
func (s *DatabaseServerV2) GetVersion() string {
	return s.Version
}

// GetTeleportVersion returns the Teleport version the server is running.
func (s *DatabaseServerV2) GetTeleportVersion() string {
	return s.Spec.Version
}

// GetHostname returns the database server hostname.
func (s *DatabaseServerV2) GetHostname() string {
	return s.Spec.Hostname
}

// GetHostID returns ID of the host the server is running on.
func (s *DatabaseServerV2) GetHostID() string {
	return s.Spec.HostID
}

// GetKind returns the resource kind.
func (s *DatabaseServerV2) GetKind() string {
	return s.Kind
}

// GetSubKind returns the resource subkind.
func (s *DatabaseServerV2) GetSubKind() string {
	return s.SubKind
}

// SetSubKind sets the resource subkind.
func (s *DatabaseServerV2) SetSubKind(sk string) {
	s.SubKind = sk
}

// GetResourceID returns the resource ID.
func (s *DatabaseServerV2) GetResourceID() int64 {
	return s.Metadata.ID
}

// SetResourceID sets the resource ID.
func (s *DatabaseServerV2) SetResourceID(id int64) {
	s.Metadata.ID = id
}

// GetMetadata returns the resource metadata.
func (s *DatabaseServerV2) GetMetadata() Metadata {
	return s.Metadata
}

// GetNamespace returns the resource namespace.
func (s *DatabaseServerV2) GetNamespace() string {
	return s.Metadata.Namespace
}

// SetExpiry sets the resource expiry time.
func (s *DatabaseServerV2) SetExpiry(expiry time.Time) {
	s.Metadata.SetExpiry(expiry)
}

// Expiry returns the resource expiry time.
func (s *DatabaseServerV2) Expiry() time.Time {
	return s.Metadata.Expiry()
}

// SetTTL sets the resource TTL.
func (s *DatabaseServerV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	s.Metadata.SetTTL(clock, ttl)
}

// GetName returns the resource name.
func (s *DatabaseServerV2) GetName() string {
	return s.Metadata.Name
}

// SetName sets the resource name.
func (s *DatabaseServerV2) SetName(name string) {
	s.Metadata.Name = name
}

// GetRotation returns the server CA rotation state.
func (s *DatabaseServerV2) GetRotation() Rotation {
	return s.Spec.Rotation
}

// SetRotation sets the server CA rotation state.
func (s *DatabaseServerV2) SetRotation(r Rotation) {
	s.Spec.Rotation = r
}

// GetStaticLabels returns the server static labels.
func (s *DatabaseServerV2) GetStaticLabels() map[string]string {
	return s.Metadata.Labels
}

// SetStaticLabels sets the server static labels.
func (s *DatabaseServerV2) SetStaticLabels(sl map[string]string) {
	s.Metadata.Labels = sl
}

// GetDynamicLabels returns the server dynamic labels.
func (s *DatabaseServerV2) GetDynamicLabels() map[string]CommandLabel {
	if s.Spec.DynamicLabels == nil {
		return nil
	}
	return V2ToLabels(s.Spec.DynamicLabels)
}

// SetDynamicLabels sets the server dynamic labels
func (s *DatabaseServerV2) SetDynamicLabels(dl map[string]CommandLabel) {
	s.Spec.DynamicLabels = LabelsToV2(dl)
}

// GetAllLabels returns combines static and dynamic labels.
func (s *DatabaseServerV2) GetAllLabels() map[string]string {
	return CombineLabels(s.Metadata.Labels, s.Spec.DynamicLabels)
}

// LabelsString returns all labels as a string.
func (s *DatabaseServerV2) LabelsString() string {
	return LabelsAsString(s.Metadata.Labels, s.Spec.DynamicLabels)
}

// GetDescription returns the database server description.
func (s *DatabaseServerV2) GetDescription() string {
	return s.Spec.Description
}

// GetProtocol returns the database server protocol.
func (s *DatabaseServerV2) GetProtocol() string {
	return s.Spec.Protocol
}

// GetURI returns the database connection address.
func (s *DatabaseServerV2) GetURI() string {
	return s.Spec.URI
}

// GetCA returns the database CA certificate bytes.
func (s *DatabaseServerV2) GetCA() []byte {
	return s.Spec.CACert
}

// GetRegion returns the AWS region for RDS/Aurora databases.
func (s *DatabaseServerV2) GetRegion() string {
	return s.Spec.AWS.Region
}

// IsAWS returns true if this database represents AWS RDS/Aurora instance.
func (s *DatabaseServerV2) IsAWS() bool {
	return s.Spec.AWS.Region != ""
}

// String returns the server string representation.
func (s *DatabaseServerV2) String() string {
	return fmt.Sprintf("DatabaseServer(Name=%v, Version=%v, Labels=%v)",
		s.GetName(), s.GetTeleportVersion(), s.GetStaticLabels())
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (s *DatabaseServerV2) CheckAndSetDefaults() error {
	if err := s.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if s.Kind == "" {
		return trace.BadParameter("database server %q kind is empty", s.GetName())
	}
	for key := range s.Spec.DynamicLabels {
		if !IsValidLabelKey(key) {
			return trace.BadParameter("database server %q invalid label key: %q", s.GetName(), key)
		}
	}
	if s.Spec.Protocol == "" {
		return trace.BadParameter("database server %q protocol is empty", s.GetName())
	}
	if s.Spec.URI == "" {
		return trace.BadParameter("database server %q URI is empty", s.GetName())
	}
	if s.Spec.Hostname == "" {
		return trace.BadParameter("database server %q hostname is empty", s.GetName())
	}
	if s.Spec.HostID == "" {
		return trace.BadParameter("database server %q host ID is empty", s.GetName())
	}
	return nil
}

// Copy returns a copy of this database server object.
func (s *DatabaseServerV2) Copy() DatabaseServer {
	return &DatabaseServerV2{
		Kind:     KindDatabaseServer,
		Version:  V2,
		Metadata: s.Metadata,
		Spec:     s.Spec,
	}
}

// CompareDatabaseServers returns whether the two provided database servers
// are equal or different.
func CompareDatabaseServers(a, b DatabaseServer) int {
	if a.GetKind() != b.GetKind() {
		return Different
	}
	if a.GetName() != b.GetName() {
		return Different
	}
	if a.GetNamespace() != b.GetNamespace() {
		return Different
	}
	if a.GetTeleportVersion() != b.GetTeleportVersion() {
		return Different
	}
	r := a.GetRotation()
	if !r.Matches(b.GetRotation()) {
		return Different
	}
	if !utils.StringMapsEqual(a.GetStaticLabels(), b.GetStaticLabels()) {
		return Different
	}
	if !CmdLabelMapsEqual(a.GetDynamicLabels(), b.GetDynamicLabels()) {
		return Different
	}
	if !a.Expiry().Equal(b.Expiry()) {
		return OnlyTimestampsDifferent
	}
	if a.GetProtocol() != b.GetProtocol() {
		return Different
	}
	if a.GetURI() != b.GetURI() {
		return Different
	}
	return Equal
}

// DatabaseServerSpecV2Schema is JSON schema for a database server spec.
const DatabaseServerSpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "description": {"type": "string"},
    "protocol": {"type": "string"},
    "uri": {"type": "string"},
    "ca_cert": {"type": "string"},
    "aws": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "region": {"type": "string"}
      }
    },
    "version": {"type": "string"},
    "hostname": {"type": "string"},
    "host_id": {"type": "string"},
    "dynamic_labels": {
      "type": "object",
      "additionalProperties": false,
      "patternProperties": {
        "^.*$": {
          "type": "object",
          "additionalProperties": false,
          "required": ["command"],
          "properties": {
            "command": {"type": "array", "items": {"type": "string"}},
            "period": {"type": "string"},
            "result": {"type": "string"}
          }
        }
      }
    },
    "rotation": %v
  }
}`

// GetDatabaseServerSchema returns full database server JSON schema.
func GetDatabaseServerSchema() string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, fmt.Sprintf(DatabaseServerSpecV2Schema, RotationSchema), DefaultDefinitions)
}

// UnmarshalDatabaseServerResource unmarshals database server resource.
func UnmarshalDatabaseServerResource(data []byte, kind string, cfg *MarshalConfig) (DatabaseServer, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing database server data")
	}
	var h ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case V2:
		var s DatabaseServerV2
		if cfg.SkipValidation {
			if err := utils.FastUnmarshal(data, &s); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		} else {
			if err := utils.UnmarshalWithSchema(GetDatabaseServerSchema(), &s, data); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		}
		s.Kind = kind
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			s.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			s.SetExpiry(cfg.Expires)
		}
		return &s, nil
	}
	return nil, trace.BadParameter("database server resource version %q is not supported", h.Version)
}

var databaseServerMarshaler DatabaseServerMarshaler = &DatabaseServerMarshalerImpl{}

// SetDatabaseServerMarshaler sets the database server resource marshaler.
func SetDatabaseServerMarshaler(m DatabaseServerMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	databaseServerMarshaler = m
}

// GetDatabaseServerMarshaler returns the database server resource marshaler.
func GetDatabaseServerMarshaler() DatabaseServerMarshaler {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	return databaseServerMarshaler
}

// DatabaseServerMarshaler implements marshal/unmarshal of database servers.
type DatabaseServerMarshaler interface {
	// UnmarshalDatabaseServer from binary representation.
	UnmarshalDatabaseServer(bytes []byte, kind string, opts ...MarshalOption) (DatabaseServer, error)
	// MarshalDatabaseServer to binary representation.
	MarshalDatabaseServer(DatabaseServer, ...MarshalOption) ([]byte, error)
	// UnmarshalDatabaseServers unmarshals multiple database servers.
	UnmarshalDatabaseServers(bytes []byte) ([]DatabaseServer, error)
	// MarshalDatabaseServers marshals multiple database servers.
	MarshalDatabaseServers([]DatabaseServer) ([]byte, error)
}

// DatabaseServerMarshalerImpl implements DatabaseServerMarshaler interface.
type DatabaseServerMarshalerImpl struct{}

// UnmarshalDatabaseServer unmarshals database server.
func (*DatabaseServerMarshalerImpl) UnmarshalDatabaseServer(bytes []byte, kind string, opts ...MarshalOption) (DatabaseServer, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return UnmarshalDatabaseServerResource(bytes, kind, cfg)
}

// MarshalDatabaseServer marshals database server.
func (*DatabaseServerMarshalerImpl) MarshalDatabaseServer(s DatabaseServer, opts ...MarshalOption) ([]byte, error) {
	if err := s.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch server := s.(type) {
	case *DatabaseServerV2:
		if !cfg.PreserveResourceID {
			copy := *server
			copy.SetResourceID(0)
			server = &copy
		}
		return utils.FastMarshal(server)
	}
	return nil, trace.BadParameter("unrecognized database server version %[1]T %[1]v", s)
}

// UnmarshalDatabaseServers unmarshals multiple database servers.
func (*DatabaseServerMarshalerImpl) UnmarshalDatabaseServers(bytes []byte) ([]DatabaseServer, error) {
	var servers []DatabaseServerV2
	if err := utils.FastUnmarshal(bytes, &servers); err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]DatabaseServer, len(servers))
	for i, v := range servers {
		out[i] = DatabaseServer(&v)
	}
	return out, nil
}

// MarshalDatabaseServers marshals multiple database servers.
func (*DatabaseServerMarshalerImpl) MarshalDatabaseServers(s []DatabaseServer) ([]byte, error) {
	bytes, err := utils.FastMarshal(s)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return bytes, nil
}
