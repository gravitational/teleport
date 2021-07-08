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
	"net"
	"strings"
	"time"

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
	// SetCA sets the database CA certificate bytes.
	SetCA([]byte)
	// GetAWS returns AWS information for RDS/Aurora databases.
	GetAWS() AWS
	// GetGCP returns GCP information for Cloud SQL databases.
	GetGCP() GCPCloudSQL
	// GetType returns the database authentication type: self-hosted, RDS, Redshift or Cloud SQL.
	GetType() string
	// IsRDS returns true if this is an RDS/Aurora database.
	IsRDS() bool
	// IsRedshift returns true if this is a Redshift database.
	IsRedshift() bool
	// IsCloudSQL returns true if this is a Cloud SQL database.
	IsCloudSQL() bool
	// Copy returns a copy of this database server object.
	Copy() DatabaseServer
}

// NewDatabaseServerV3 creates a new database server instance.
func NewDatabaseServerV3(name string, labels map[string]string, spec DatabaseServerSpecV3) (*DatabaseServerV3, error) {
	s := &DatabaseServerV3{
		Metadata: Metadata{
			Name:   name,
			Labels: labels,
		},
		Spec: spec,
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

// SetCA sets the database CA certificate bytes.
func (s *DatabaseServerV3) SetCA(bytes []byte) {
	s.Spec.CACert = bytes
}

// GetAWS returns AWS information for RDS/Aurora databases.
func (s *DatabaseServerV3) GetAWS() AWS {
	return s.Spec.AWS
}

// GetGCP returns GCP information for Cloud SQL databases.
func (s *DatabaseServerV3) GetGCP() GCPCloudSQL {
	return s.Spec.GCP
}

// IsRDS returns true if this database represents AWS RDS/Aurora instance.
func (s *DatabaseServerV3) IsRDS() bool {
	return s.GetType() == DatabaseTypeRDS
}

// IsRedshift returns true if this is a Redshift database instance.
func (s *DatabaseServerV3) IsRedshift() bool {
	return s.GetType() == DatabaseTypeRedshift
}

// IsCloudSQL returns true if this database is a Cloud SQL instance.
func (s *DatabaseServerV3) IsCloudSQL() bool {
	return s.GetType() == DatabaseTypeCloudSQL
}

// GetType returns the database type, self-hosted or AWS RDS.
func (s *DatabaseServerV3) GetType() string {
	if s.Spec.AWS.Redshift.ClusterID != "" {
		return DatabaseTypeRedshift
	}
	if s.Spec.AWS.Region != "" {
		return DatabaseTypeRDS
	}
	if s.Spec.GCP.ProjectID != "" {
		return DatabaseTypeCloudSQL
	}
	return DatabaseTypeSelfHosted
}

// String returns the server string representation.
func (s *DatabaseServerV3) String() string {
	return fmt.Sprintf("DatabaseServer(Name=%v, Type=%v, Version=%v, Labels=%v, HostID=%v)",
		s.GetName(), s.GetType(), s.GetTeleportVersion(), s.GetStaticLabels(), s.Spec.HostID)
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
	// In case of RDS, Aurora or Redshift, AWS information such as region or
	// cluster ID can be extracted from the endpoint if not provided.
	switch {
	case strings.Contains(s.Spec.URI, rdsEndpointSuffix):
		region, err := parseRDSEndpoint(s.Spec.URI)
		if err != nil {
			return trace.Wrap(err)
		}
		if s.Spec.AWS.Region == "" {
			s.Spec.AWS.Region = region
		}
	case strings.Contains(s.Spec.URI, redshiftEndpointSuffix):
		clusterID, region, err := parseRedshiftEndpoint(s.Spec.URI)
		if err != nil {
			return trace.Wrap(err)
		}
		if s.Spec.AWS.Redshift.ClusterID == "" {
			s.Spec.AWS.Redshift.ClusterID = clusterID
		}
		if s.Spec.AWS.Region == "" {
			s.Spec.AWS.Region = region
		}
	}
	return nil
}

// parseRDSEndpoint extracts region from the provided RDS endpoint.
func parseRDSEndpoint(endpoint string) (region string, err error) {
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", trace.Wrap(err)
	}
	// RDS/Aurora endpoint looks like this:
	// aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com
	parts := strings.Split(host, ".")
	if !strings.HasSuffix(host, rdsEndpointSuffix) || len(parts) != 6 {
		return "", trace.BadParameter("failed to parse %v as RDS endpoint", endpoint)
	}
	return parts[2], nil
}

// parseRedshiftEndpoint extracts cluster ID and region from the provided Redshift endpoint.
func parseRedshiftEndpoint(endpoint string) (clusterID, region string, err error) {
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	// Redshift endpoint looks like this:
	// redshift-cluster-1.abcdefghijklmnop.us-east-1.rds.amazonaws.com
	parts := strings.Split(host, ".")
	if !strings.HasSuffix(host, redshiftEndpointSuffix) || len(parts) != 6 {
		return "", "", trace.BadParameter("failed to parse %v as Redshift endpoint", endpoint)
	}
	return parts[0], parts[2], nil
}

// Copy returns a copy of this database server object.
func (s *DatabaseServerV3) Copy() DatabaseServer {
	return proto.Clone(s).(*DatabaseServerV3)
}

const (
	// DatabaseTypeSelfHosted is the self-hosted type of database.
	DatabaseTypeSelfHosted = "self-hosted"
	// DatabaseTypeRDS is AWS-hosted RDS or Aurora database.
	DatabaseTypeRDS = "rds"
	// DatabaseTypeRedshift is AWS Redshift database.
	DatabaseTypeRedshift = "redshift"
	// DatabaseTypeCloudSQL is GCP-hosted Cloud SQL database.
	DatabaseTypeCloudSQL = "gcp"
)

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

// DatabaseServers is a list of database servers.
type DatabaseServers []DatabaseServer

// DeduplicateDatabaseServers deduplicates database servers by name.
func DeduplicateDatabaseServers(servers []DatabaseServer) (result []DatabaseServer) {
	seen := make(map[string]struct{})
	for _, server := range servers {
		if _, ok := seen[server.GetName()]; ok {
			continue
		}
		seen[server.GetName()] = struct{}{}
		result = append(result, server)
	}
	return result
}

const (
	// rdsEndpointSuffix is the RDS/Aurora endpoint suffix.
	rdsEndpointSuffix = ".rds.amazonaws.com"
	// redshiftEndpointSuffix is the Redshift endpoint suffix.
	redshiftEndpointSuffix = ".redshift.amazonaws.com"
)
