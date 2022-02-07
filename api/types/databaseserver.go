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
	// ResourceWithLabels provides common resource methods.
	ResourceWithLabels
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
	// GetDatabase returns the database this database server proxies.
	GetDatabase() Database
	// SetDatabase sets the database this database server proxies.
	SetDatabase(Database) error
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

// GetDatabase returns the database this database server proxies.
func (s *DatabaseServerV3) GetDatabase() Database {
	if s.Spec.Database != nil {
		return s.Spec.Database
	}
	// If any older database agents are still heartbeating back, they have
	// fields like protocol, URI, etc. set in the DatabaseServer object
	// itself, so construct the Database object from them.
	//
	// DELETE IN 9.0.
	return &DatabaseV3{
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
	}
}

// SetDatabase sets the database this database server proxies.
func (s *DatabaseServerV3) SetDatabase(database Database) error {
	databaseV3, ok := database.(*DatabaseV3)
	if !ok {
		return trace.BadParameter("expected *DatabaseV3, got %T", database)
	}
	s.Spec.Database = databaseV3
	return nil
}

// String returns the server string representation.
func (s *DatabaseServerV3) String() string {
	return fmt.Sprintf("DatabaseServer(Name=%v, Version=%v, Hostname=%v, HostID=%v, Database=%v)",
		s.GetName(), s.GetTeleportVersion(), s.GetHostname(), s.GetHostID(), s.GetDatabase())
}

// setStaticFields sets static resource header and metadata fields.
func (s *DatabaseServerV3) setStaticFields() {
	s.Kind = KindDatabaseServer
	s.Version = V3
}

// setLegacyFields sets fields that database servers used to have before
// Database resource became a separate field.
//
// This is required for backwards compatibility in case a database agent
// connects back to a pre-8.0 auth server.
//
// DELETE IN 9.0.
func (s *DatabaseServerV3) setLegacyFields(database *DatabaseV3) {
	s.Metadata.Labels = database.Metadata.Labels
	s.Spec.Description = database.Metadata.Description
	s.Spec.Protocol = database.Spec.Protocol
	s.Spec.URI = database.Spec.URI
	s.Spec.CACert = []byte(database.Spec.CACert)
	s.Spec.DynamicLabels = database.Spec.DynamicLabels
	s.Spec.AWS = database.Spec.AWS
	s.Spec.GCP = database.Spec.GCP
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
	if s.Spec.Database != nil {
		if err := s.Spec.Database.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		s.setLegacyFields(s.Spec.Database)
	}
	return nil
}

// Origin returns the origin value of the resource.
func (s *DatabaseServerV3) Origin() string {
	return s.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (s *DatabaseServerV3) SetOrigin(origin string) {
	s.Metadata.SetOrigin(origin)
}

// GetAllLabels returns all resource's labels. Considering:
// * Static labels from `Metadata.Labels` and `Spec.Database`.
// * Dynamic labels from `Spec.DynamicLabels`.
func (s *DatabaseServerV3) GetAllLabels() map[string]string {
	staticLabels := make(map[string]string)
	for name, value := range s.Metadata.Labels {
		staticLabels[name] = value
	}

	if s.Spec.Database != nil {
		for name, value := range s.Spec.Database.Metadata.Labels {
			staticLabels[name] = value
		}
	}

	return CombineLabels(staticLabels, s.Spec.DynamicLabels)
}

// Copy returns a copy of this database server object.
func (s *DatabaseServerV3) Copy() DatabaseServer {
	return proto.Clone(s).(*DatabaseServerV3)
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (s *DatabaseServerV3) MatchSearch(values []string) bool {
	return MatchSearch(nil, values, nil)
}

// DatabaseServers represents a list of database servers.
type DatabaseServers []DatabaseServer

// Len returns the slice length.
func (s DatabaseServers) Len() int { return len(s) }

// Less compares database servers by name and host ID.
func (s DatabaseServers) Less(i, j int) bool {
	return s[i].GetName() < s[j].GetName() && s[i].GetHostID() < s[j].GetHostID()
}

// Swap swaps two database servers.
func (s DatabaseServers) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
