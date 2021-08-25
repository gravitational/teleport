/*
Copyright 2020-2021 Gravitational, Inc.

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

	"github.com/gravitational/teleport/api"

	"github.com/gogo/protobuf/proto"
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
	// GetRotation gets the state of certificate authority rotation.
	GetRotation() Rotation
	// SetRotation sets the state of certificate authority rotation.
	SetRotation(Rotation)
	// String returns string representation of the server.
	String() string
	// Copy returns a copy of this database server object.
	Copy() DatabaseServer
	// GetDatabases returns databases this database server proxies.
	GetDatabases() []Database
	// SetDatabases sets databases this database server proxies.
	SetDatabases([]Database) error
}

// NewDatabaseServerV3 creates a new database server instance.
func NewDatabaseServerV3(meta Metadata, spec DatabaseServerSpecV3) (*DatabaseServerV3, error) {
	s := &DatabaseServerV3{
		Metadata: meta,
		Spec:     spec,
	}
	if err := s.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return s, nil
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

// GetDatabases returns databases this database server proxies.
func (s *DatabaseServerV3) GetDatabases() (databases []Database) {
	if len(s.Spec.Databases) > 0 {
		for _, database := range s.Spec.Databases {
			databases = append(databases, database)
		}
	} else if s.Spec.URI != "" {
		// Database server used to represent a single database in 7 and before,
		// so in case any older database agents are still heartbeating back,
		// adapt them to the current interface where server has a list of
		// databases it proxies.
		//
		// DELETE IN 9.0.
		databases = append(databases, &DatabaseV3{
			Kind:    KindDatabase,
			Version: V3,
			Metadata: Metadata{
				Name:        s.Metadata.Name,
				Namespace:   s.Metadata.Namespace,
				Description: s.Spec.Description,
				Labels:      s.Metadata.Labels,
			},
			Spec: DatabaseSpecV3{
				Protocol:      s.Spec.Protocol,
				URI:           s.Spec.URI,
				CACert:        string(s.Spec.CACert),
				DynamicLabels: s.Spec.DynamicLabels,
				AWS:           s.Spec.AWS,
				GCP:           s.Spec.GCP,
			},
		})
	}
	return databases
}

// SetDatabases sets databases this database server proxies.
func (s *DatabaseServerV3) SetDatabases(databases []Database) error {
	databasesV3 := make([]*DatabaseV3, 0, len(databases))
	for _, database := range databases {
		databaseV3, ok := database.(*DatabaseV3)
		if !ok {
			return trace.BadParameter("expected *DatabaseV3, got %T", database)
		}
		databasesV3 = append(databasesV3, databaseV3)
	}
	s.Spec.Databases = databasesV3
	return nil
}

// String returns the server string representation.
func (s *DatabaseServerV3) String() string {
	return fmt.Sprintf("DatabaseServer(Name=%v, Version=%v, Hostname=%v, HostID=%v, Databases=%v)",
		s.GetName(), s.GetTeleportVersion(), s.GetHostname(), s.GetHostID(), s.GetDatabases())
}

// setStaticFields sets static resource header and metadata fields.
func (s *DatabaseServerV3) setStaticFields() {
	s.Kind = KindDatabaseServer
	s.Version = V3
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (s *DatabaseServerV3) CheckAndSetDefaults() error {
	s.setStaticFields()
	if err := s.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if s.Spec.HostID == "" {
		return trace.BadParameter("missing database server HostID")
	}
	if s.Spec.Hostname == "" {
		return trace.BadParameter("missing database server Hostname")
	}
	if s.Spec.Version == "" {
		s.Spec.Version = api.Version
	}
	for i := range s.Spec.Databases {
		if err := s.Spec.Databases[i].CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Copy returns a copy of this database server object.
func (s *DatabaseServerV3) Copy() DatabaseServer {
	return proto.Clone(s).(*DatabaseServerV3)
}

// SortedDatabaseServers implements sorter for database servers.
type SortedDatabaseServers []DatabaseServer

// Len returns the slice length.
func (s SortedDatabaseServers) Len() int { return len(s) }

// Less compares database servers by name and host ID.
func (s SortedDatabaseServers) Less(i, j int) bool {
	return s[i].GetName() < s[j].GetName() && s[i].GetHostID() < s[j].GetHostID()
}

// Swap swaps two database servers.
func (s SortedDatabaseServers) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
