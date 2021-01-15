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

package types

import (
	"fmt"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
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
	// GetType returns the database type, self-hosted or AWS RDS.
	GetType() string
	// IsRDS returns true if this an RDS/Aurora database.
	IsRDS() bool
	// CheckAndSetDefaults checks and set default values for any missing fields.
	CheckAndSetDefaults() error
	// Copy returns a copy of this database server object.
	Copy() DatabaseServer
}

// NewDatabaseServerV3 creates a new database server instance.
func NewDatabaseServerV3(name string, labels map[string]string, spec DatabaseServerSpecV3) *DatabaseServerV3 {
	return &DatabaseServerV3{
		Kind:    KindDatabaseServer,
		Version: V3,
		Metadata: Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
			Labels:    labels,
		},
		Spec: spec,
	}
}

// GetVersion returns the database server resource version.
func (s *DatabaseServerV3) GetVersion() string {
	return s.Version
}

// GetTeleportVersion returns the Teleport version the server is running.
func (s *DatabaseServerV3) GetTeleportVersion() string {
	return s.Spec.Version
}

// GetHostname returns the database server hostname.
func (s *DatabaseServerV3) GetHostname() string {
	return s.Spec.Hostname
}

// GetHostID returns ID of the host the server is running on.
func (s *DatabaseServerV3) GetHostID() string {
	return s.Spec.HostID
}

// GetKind returns the resource kind.
func (s *DatabaseServerV3) GetKind() string {
	return s.Kind
}

// GetSubKind returns the resource subkind.
func (s *DatabaseServerV3) GetSubKind() string {
	return s.SubKind
}

// SetSubKind sets the resource subkind.
func (s *DatabaseServerV3) SetSubKind(sk string) {
	s.SubKind = sk
}

// GetResourceID returns the resource ID.
func (s *DatabaseServerV3) GetResourceID() int64 {
	return s.Metadata.ID
}

// SetResourceID sets the resource ID.
func (s *DatabaseServerV3) SetResourceID(id int64) {
	s.Metadata.ID = id
}

// GetMetadata returns the resource metadata.
func (s *DatabaseServerV3) GetMetadata() Metadata {
	return s.Metadata
}

// GetNamespace returns the resource namespace.
func (s *DatabaseServerV3) GetNamespace() string {
	return s.Metadata.Namespace
}

// SetExpiry sets the resource expiry time.
func (s *DatabaseServerV3) SetExpiry(expiry time.Time) {
	s.Metadata.SetExpiry(expiry)
}

// Expiry returns the resource expiry time.
func (s *DatabaseServerV3) Expiry() time.Time {
	return s.Metadata.Expiry()
}

// SetTTL sets the resource TTL.
func (s *DatabaseServerV3) SetTTL(clock Clock, ttl time.Duration) {
	s.Metadata.SetTTL(clock, ttl)
}

// GetName returns the resource name.
func (s *DatabaseServerV3) GetName() string {
	return s.Metadata.Name
}

// SetName sets the resource name.
func (s *DatabaseServerV3) SetName(name string) {
	s.Metadata.Name = name
}

// GetRotation returns the server CA rotation state.
func (s *DatabaseServerV3) GetRotation() Rotation {
	return s.Spec.Rotation
}

// SetRotation sets the server CA rotation state.
func (s *DatabaseServerV3) SetRotation(r Rotation) {
	s.Spec.Rotation = r
}

// GetStaticLabels returns the server static labels.
func (s *DatabaseServerV3) GetStaticLabels() map[string]string {
	return s.Metadata.Labels
}

// SetStaticLabels sets the server static labels.
func (s *DatabaseServerV3) SetStaticLabels(sl map[string]string) {
	s.Metadata.Labels = sl
}

// GetDynamicLabels returns the server dynamic labels.
func (s *DatabaseServerV3) GetDynamicLabels() map[string]CommandLabel {
	if s.Spec.DynamicLabels == nil {
		return nil
	}
	return V2ToLabels(s.Spec.DynamicLabels)
}

// SetDynamicLabels sets the server dynamic labels
func (s *DatabaseServerV3) SetDynamicLabels(dl map[string]CommandLabel) {
	s.Spec.DynamicLabels = LabelsToV2(dl)
}

// GetAllLabels returns combines static and dynamic labels.
func (s *DatabaseServerV3) GetAllLabels() map[string]string {
	return CombineLabels(s.Metadata.Labels, s.Spec.DynamicLabels)
}

// LabelsString returns all labels as a string.
func (s *DatabaseServerV3) LabelsString() string {
	return LabelsAsString(s.Metadata.Labels, s.Spec.DynamicLabels)
}

// GetDescription returns the database server description.
func (s *DatabaseServerV3) GetDescription() string {
	return s.Spec.Description
}

// GetProtocol returns the database server protocol.
func (s *DatabaseServerV3) GetProtocol() string {
	return s.Spec.Protocol
}

// GetURI returns the database connection address.
func (s *DatabaseServerV3) GetURI() string {
	return s.Spec.URI
}

// GetCA returns the database CA certificate bytes.
func (s *DatabaseServerV3) GetCA() []byte {
	return s.Spec.CACert
}

// GetRegion returns the AWS region for RDS/Aurora databases.
func (s *DatabaseServerV3) GetRegion() string {
	return s.Spec.AWS.Region
}

// IsRDS returns true if this database represents AWS RDS/Aurora instance.
func (s *DatabaseServerV3) IsRDS() bool {
	return s.GetType() == DatabaseTypeRDS
}

// GetType returns the database type, self-hosted or AWS RDS.
func (s *DatabaseServerV3) GetType() string {
	if s.Spec.AWS.Region != "" {
		return DatabaseTypeRDS
	}
	return DatabaseTypeSelfHosted
}

// String returns the server string representation.
func (s *DatabaseServerV3) String() string {
	return fmt.Sprintf("DatabaseServer(Name=%v, Version=%v, Labels=%v)",
		s.GetName(), s.GetTeleportVersion(), s.GetStaticLabels())
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (s *DatabaseServerV3) CheckAndSetDefaults() error {
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
func (s *DatabaseServerV3) Copy() DatabaseServer {
	return &DatabaseServerV3{
		Kind:     KindDatabaseServer,
		Version:  V3,
		Metadata: s.Metadata,
		Spec:     s.Spec,
	}
}

// DatabaseServerSpecV3Schema is JSON schema for a database server spec.
const DatabaseServerSpecV3Schema = `{
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
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, fmt.Sprintf(DatabaseServerSpecV3Schema, RotationSchema), DefaultDefinitions)
}

// MarshalDatabaseServer marshals the database server resource.
func MarshalDatabaseServer(s DatabaseServer, opts ...MarshalOption) ([]byte, error) {
	if err := s.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	server := s
	if !cfg.PreserveResourceID {
		// Avoid modifying the original object to prevent unexpected
		// data races.
		server = s.Copy()
		server.SetResourceID(0)
	}
	return utils.FastMarshal(server)
}

// UnmarshalDatabaseServer unmarshals the database server resource.
func UnmarshalDatabaseServer(data []byte, opts ...MarshalOption) (DatabaseServer, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing database server data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case V3:
		var s DatabaseServerV3
		if cfg.SkipValidation {
			if err := utils.FastUnmarshal(data, &s); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		} else {
			if err := utils.UnmarshalWithSchema(GetDatabaseServerSchema(), &s, data); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		}
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

const (
	// DatabaseTypeSelfHosted is the self-hosted type of database.
	DatabaseTypeSelfHosted = "self-hosted"
	// DatabaseTypeRDS is AWS-hosted RDS or Aurora database.
	DatabaseTypeRDS = "rds"
)
