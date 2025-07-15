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
	"maps"
	"sort"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/utils"
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

	// CloneResource returns a copy of the DatabaseServer as a ResourceWithLabels
	CloneResource() ResourceWithLabels
	// GetDatabase returns the database this database server proxies.
	GetDatabase() Database
	// SetDatabase sets the database this database server proxies.
	SetDatabase(Database) error
	// ProxiedService provides common methods for a proxied service.
	ProxiedService
	// GetTargetHealth returns the database server's target health.
	GetTargetHealth() TargetHealth
	// SetTargetHealth sets the database server's target health.
	SetTargetHealth(h TargetHealth)
	// GetTargetHealthStatus returns target health status
	GetTargetHealthStatus() TargetHealthStatus
	// SetTargetHealthStatus sets target health status
	SetTargetHealthStatus(status TargetHealthStatus)
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

// GetRevision returns the revision
func (s *DatabaseServerV3) GetRevision() string {
	return s.Metadata.GetRevision()
}

// SetRevision sets the revision
func (s *DatabaseServerV3) SetRevision(rev string) {
	s.Metadata.SetRevision(rev)
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
	if s.Spec.Database == nil {
		return nil
	}
	return s.Spec.Database
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

// GetProxyID returns a list of proxy ids this server is connected to.
func (s *DatabaseServerV3) GetProxyIDs() []string {
	return s.Spec.ProxyIDs
}

// SetProxyID sets the proxy ids this server is connected to.
func (s *DatabaseServerV3) SetProxyIDs(proxyIDs []string) {
	s.Spec.ProxyIDs = proxyIDs
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

	if s.Spec.Database == nil {
		return trace.BadParameter("missing database server Database")
	}

	if err := s.Spec.Database.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
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

// GetLabel retrieves the label with the provided key. If not found
// value will be empty and ok will be false.
func (s *DatabaseServerV3) GetLabel(key string) (value string, ok bool) {
	if s.Spec.Database != nil {
		if v, ok := s.Spec.Database.GetLabel(key); ok {
			return v, ok
		}
	}

	v, ok := s.Metadata.Labels[key]
	return v, ok
}

// GetAllLabels returns all resource's labels. Considering:
// * Static labels from `Metadata.Labels` and `Spec.Database`.
// * Dynamic labels from `Spec.DynamicLabels`.
func (s *DatabaseServerV3) GetAllLabels() map[string]string {
	staticLabels := map[string]string{}
	maps.Copy(staticLabels, s.Metadata.Labels)
	if s.Spec.Database != nil {
		maps.Copy(staticLabels, s.Spec.Database.GetAllLabels())
	}

	return staticLabels
}

// GetStaticLabels returns the database server static labels.
func (s *DatabaseServerV3) GetStaticLabels() map[string]string {
	return s.Metadata.Labels
}

// SetStaticLabels sets the database server static labels.
func (s *DatabaseServerV3) SetStaticLabels(sl map[string]string) {
	s.Metadata.Labels = sl
}

// Copy returns a copy of this database server object.
func (s *DatabaseServerV3) Copy() DatabaseServer {
	return utils.CloneProtoMsg(s)
}

// CloneResource returns a copy of this database server object.
func (s *DatabaseServerV3) CloneResource() ResourceWithLabels {
	return s.Copy()
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (s *DatabaseServerV3) MatchSearch(values []string) bool {
	return MatchSearch(nil, values, nil)
}

// GetTargetHealth returns the database server's target health.
func (s *DatabaseServerV3) GetTargetHealth() TargetHealth {
	if s.Status.TargetHealth == nil {
		return TargetHealth{}
	}
	return *s.Status.TargetHealth
}

// SetTargetHealth sets the database server's target health status.
func (s *DatabaseServerV3) SetTargetHealth(h TargetHealth) {
	s.Status.TargetHealth = &h
}

// GetTargetHealthStatus returns target health status
func (s *DatabaseServerV3) GetTargetHealthStatus() TargetHealthStatus {
	if s.Status.TargetHealth == nil {
		return ""
	}
	return TargetHealthStatus(s.Status.TargetHealth.Status)
}

// SetTargetHealthStatus sets target health status
func (s *DatabaseServerV3) SetTargetHealthStatus(status TargetHealthStatus) {
	if s.Status.TargetHealth == nil {
		s.Status.TargetHealth = &TargetHealth{}
	}
	s.Status.TargetHealth.Status = string(status)
}

// DatabaseServers represents a list of database servers.
type DatabaseServers []DatabaseServer

// Len returns the slice length.
func (s DatabaseServers) Len() int { return len(s) }

// Less compares database servers by name and host ID.
func (s DatabaseServers) Less(i, j int) bool {
	switch {
	case s[i].GetName() < s[j].GetName():
		return true
	case s[i].GetName() > s[j].GetName():
		return false
	default:
		return s[i].GetHostID() < s[j].GetHostID()
	}
}

// Swap swaps two database servers.
func (s DatabaseServers) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// SortByCustom custom sorts by given sort criteria.
func (s DatabaseServers) SortByCustom(sortBy SortBy) error {
	if sortBy.Field == "" {
		return nil
	}

	// We assume sorting by type DatabaseServer, we are really
	// wanting to sort its contained resource Database.
	isDesc := sortBy.IsDesc
	switch sortBy.Field {
	case ResourceMetadataName:
		sort.SliceStable(s, func(i, j int) bool {
			return stringCompare(s[i].GetDatabase().GetName(), s[j].GetDatabase().GetName(), isDesc)
		})
	case ResourceSpecDescription:
		sort.SliceStable(s, func(i, j int) bool {
			return stringCompare(s[i].GetDatabase().GetDescription(), s[j].GetDatabase().GetDescription(), isDesc)
		})
	case ResourceSpecType:
		sort.SliceStable(s, func(i, j int) bool {
			return stringCompare(s[i].GetDatabase().GetType(), s[j].GetDatabase().GetType(), isDesc)
		})
	default:
		return trace.NotImplemented("sorting by field %q for resource %q is not supported", sortBy.Field, KindDatabaseServer)
	}

	return nil
}

// AsResources returns db servers as type resources with labels.
func (s DatabaseServers) AsResources() []ResourceWithLabels {
	resources := make([]ResourceWithLabels, 0, len(s))
	for _, server := range s {
		resources = append(resources, ResourceWithLabels(server))
	}
	return resources
}

// GetFieldVals returns list of select field values.
func (s DatabaseServers) GetFieldVals(field string) ([]string, error) {
	vals := make([]string, 0, len(s))
	switch field {
	case ResourceMetadataName:
		for _, server := range s {
			vals = append(vals, server.GetDatabase().GetName())
		}
	case ResourceSpecDescription:
		for _, server := range s {
			vals = append(vals, server.GetDatabase().GetDescription())
		}
	case ResourceSpecType:
		for _, server := range s {
			vals = append(vals, server.GetDatabase().GetType())
		}
	default:
		return nil, trace.NotImplemented("getting field %q for resource %q is not supported", field, KindDatabaseServer)
	}

	return vals, nil
}

// ToDatabases converts database servers to a list of databases and
// deduplicates the databases by name.
func (s DatabaseServers) ToDatabases() []Database {
	databases := make([]Database, 0, len(s))
	for _, server := range s {
		databases = append(databases, server.GetDatabase())
	}
	return DeduplicateDatabases(databases)
}
