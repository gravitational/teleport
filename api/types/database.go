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

package types

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/trace"
)

// Database represents a database proxied by a database server.
type Database interface {
	// ResourceWithLabels provides common resource methods.
	ResourceWithLabels
	// GetNamespace returns the database namespace.
	GetNamespace() string
	// GetStaticLabels returns the database static labels.
	GetStaticLabels() map[string]string
	// SetStaticLabels sets the database static labels.
	SetStaticLabels(map[string]string)
	// GetDynamicLabels returns the database dynamic labels.
	GetDynamicLabels() map[string]CommandLabel
	// SetDynamicLabels sets the database dynamic labels.
	SetDynamicLabels(map[string]CommandLabel)
	// LabelsString returns all labels as a string.
	LabelsString() string
	// String returns string representation of the database.
	String() string
	// GetDescription returns the database description.
	GetDescription() string
	// GetProtocol returns the database protocol.
	GetProtocol() string
	// GetURI returns the database connection endpoint.
	GetURI() string
	// SetURI sets the database connection endpoint.
	SetURI(string)
	// GetCA returns the database CA certificate.
	GetCA() string
	// GetTLS returns the database TLS configuration.
	GetTLS() DatabaseTLS
	// SetStatusCA sets the database CA certificate in the status field.
	SetStatusCA(string)
	// GetAWS returns the database AWS metadata.
	GetAWS() AWS
	// SetStatusAWS sets the database AWS metadata in the status field.
	SetStatusAWS(AWS)
	// GetGCP returns GCP information for Cloud SQL databases.
	GetGCP() GCPCloudSQL
	// GetAzure returns Azure database server metadata.
	GetAzure() Azure
	// GetType returns the database authentication type: self-hosted, RDS, Redshift or Cloud SQL.
	GetType() string
	// GetIAMPolicy returns AWS IAM policy for the database.
	GetIAMPolicy() string
	// GetIAMAction returns AWS IAM action needed to connect to the database.
	GetIAMAction() string
	// GetIAMResources returns AWS IAM resources that provide access to the database.
	GetIAMResources() []string
	// IsRDS returns true if this is an RDS/Aurora database.
	IsRDS() bool
	// IsRedshift returns true if this is a Redshift database.
	IsRedshift() bool
	// IsCloudSQL returns true if this is a Cloud SQL database.
	IsCloudSQL() bool
	// IsAzure returns true if this is an Azure database.
	IsAzure() bool
	// IsCloudHosted returns true if database is hosted in the cloud (AWS RDS/Aurora/Redshift, Azure or Cloud SQL).
	IsCloudHosted() bool
	// Copy returns a copy of this database resource.
	Copy() *DatabaseV3
}

// NewDatabaseV3 creates a new database resource.
func NewDatabaseV3(meta Metadata, spec DatabaseSpecV3) (*DatabaseV3, error) {
	database := &DatabaseV3{
		Metadata: meta,
		Spec:     spec,
	}
	if err := database.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return database, nil
}

// GetVersion returns the database resource version.
func (d *DatabaseV3) GetVersion() string {
	return d.Version
}

// GetKind returns the database resource kind.
func (d *DatabaseV3) GetKind() string {
	return d.Kind
}

// GetSubKind returns the database resource subkind.
func (d *DatabaseV3) GetSubKind() string {
	return d.SubKind
}

// SetSubKind sets the database resource subkind.
func (d *DatabaseV3) SetSubKind(sk string) {
	d.SubKind = sk
}

// GetResourceID returns the database resource ID.
func (d *DatabaseV3) GetResourceID() int64 {
	return d.Metadata.ID
}

// SetResourceID sets the database resource ID.
func (d *DatabaseV3) SetResourceID(id int64) {
	d.Metadata.ID = id
}

// GetMetadata returns the database resource metadata.
func (d *DatabaseV3) GetMetadata() Metadata {
	return d.Metadata
}

// Origin returns the origin value of the resource.
func (d *DatabaseV3) Origin() string {
	return d.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (d *DatabaseV3) SetOrigin(origin string) {
	d.Metadata.SetOrigin(origin)
}

// GetNamespace returns the database resource namespace.
func (d *DatabaseV3) GetNamespace() string {
	return d.Metadata.Namespace
}

// SetExpiry sets the database resource expiration time.
func (d *DatabaseV3) SetExpiry(expiry time.Time) {
	d.Metadata.SetExpiry(expiry)
}

// Expiry returns the database resource expiration time.
func (d *DatabaseV3) Expiry() time.Time {
	return d.Metadata.Expiry()
}

// GetName returns the database resource name.
func (d *DatabaseV3) GetName() string {
	return d.Metadata.Name
}

// SetName sets the database resource name.
func (d *DatabaseV3) SetName(name string) {
	d.Metadata.Name = name
}

// GetStaticLabels returns the database static labels.
func (d *DatabaseV3) GetStaticLabels() map[string]string {
	return d.Metadata.Labels
}

// SetStaticLabels sets the database static labels.
func (d *DatabaseV3) SetStaticLabels(sl map[string]string) {
	d.Metadata.Labels = sl
}

// GetDynamicLabels returns the database dynamic labels.
func (d *DatabaseV3) GetDynamicLabels() map[string]CommandLabel {
	if d.Spec.DynamicLabels == nil {
		return nil
	}
	return V2ToLabels(d.Spec.DynamicLabels)
}

// SetDynamicLabels sets the database dynamic labels
func (d *DatabaseV3) SetDynamicLabels(dl map[string]CommandLabel) {
	d.Spec.DynamicLabels = LabelsToV2(dl)
}

// GetAllLabels returns the database combined static and dynamic labels.
func (d *DatabaseV3) GetAllLabels() map[string]string {
	return CombineLabels(d.Metadata.Labels, d.Spec.DynamicLabels)
}

// LabelsString returns all database labels as a string.
func (d *DatabaseV3) LabelsString() string {
	return LabelsAsString(d.Metadata.Labels, d.Spec.DynamicLabels)
}

// GetDescription returns the database description.
func (d *DatabaseV3) GetDescription() string {
	return d.Metadata.Description
}

// GetProtocol returns the database protocol.
func (d *DatabaseV3) GetProtocol() string {
	return d.Spec.Protocol
}

// GetURI returns the database connection address.
func (d *DatabaseV3) GetURI() string {
	return d.Spec.URI
}

// SetURI sets the database connection address.
func (d *DatabaseV3) SetURI(uri string) {
	d.Spec.URI = uri
}

// GetCA returns the database CA certificate. If more than one CA is set, then
// the user provided CA is returned first (Spec field).
// Auto-downloaded CA certificate is returned otherwise.
func (d *DatabaseV3) GetCA() string {
	if d.Spec.TLS.CACert != "" {
		return d.Spec.TLS.CACert
	}
	if d.Spec.CACert != "" {
		return d.Spec.CACert
	}
	return d.Status.CACert
}

// GetTLS returns Database TLS configuration.
func (d *DatabaseV3) GetTLS() DatabaseTLS {
	return d.Spec.TLS
}

// SetStatusCA sets the database CA certificate in the status field.
func (d *DatabaseV3) SetStatusCA(ca string) {
	d.Status.CACert = ca
}

// IsEmpty returns true if AWS metadata is empty.
func (a AWS) IsEmpty() bool {
	return cmp.Equal(a, AWS{})
}

// GetAWS returns the database AWS metadata.
func (d *DatabaseV3) GetAWS() AWS {
	if !d.Status.AWS.IsEmpty() {
		return d.Status.AWS
	}
	return d.Spec.AWS
}

// SetStatusAWS sets the database AWS metadata in the status field.
func (d *DatabaseV3) SetStatusAWS(aws AWS) {
	d.Status.AWS = aws
}

// GetGCP returns GCP information for Cloud SQL databases.
func (d *DatabaseV3) GetGCP() GCPCloudSQL {
	return d.Spec.GCP
}

// GetAzure returns Azure database server metadata.
func (d *DatabaseV3) GetAzure() Azure {
	return d.Spec.Azure
}

// IsRDS returns true if this is an AWS RDS/Aurora instance.
func (d *DatabaseV3) IsRDS() bool {
	return d.GetType() == DatabaseTypeRDS
}

// IsRedshift returns true if this is a Redshift database instance.
func (d *DatabaseV3) IsRedshift() bool {
	return d.GetType() == DatabaseTypeRedshift
}

// IsCloudSQL returns true if this database is a Cloud SQL instance.
func (d *DatabaseV3) IsCloudSQL() bool {
	return d.GetType() == DatabaseTypeCloudSQL
}

// IsAzure returns true if this is Azure hosted database.
func (d *DatabaseV3) IsAzure() bool {
	return d.GetType() == DatabaseTypeAzure
}

// IsCloudHosted returns true if database is hosted in the cloud (AWS RDS/Aurora/Redshift, Azure or Cloud SQL).
func (d *DatabaseV3) IsCloudHosted() bool {
	return d.IsRDS() || d.IsRedshift() || d.IsCloudSQL() || d.IsAzure()
}

// GetType returns the database type.
func (d *DatabaseV3) GetType() string {
	if d.GetAWS().Redshift.ClusterID != "" {
		return DatabaseTypeRedshift
	}
	if d.GetAWS().Region != "" || d.GetAWS().RDS.InstanceID != "" || d.GetAWS().RDS.ClusterID != "" {
		return DatabaseTypeRDS
	}
	if d.GetGCP().ProjectID != "" {
		return DatabaseTypeCloudSQL
	}
	if d.GetAzure().Name != "" {
		return DatabaseTypeAzure
	}
	return DatabaseTypeSelfHosted
}

// String returns the database string representation.
func (d *DatabaseV3) String() string {
	return fmt.Sprintf("Database(Name=%v, Type=%v, Labels=%v)",
		d.GetName(), d.GetType(), d.GetAllLabels())
}

// Copy returns a copy of this database resource.
func (d *DatabaseV3) Copy() *DatabaseV3 {
	return proto.Clone(d).(*DatabaseV3)
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (d *DatabaseV3) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(d.GetAllLabels()), d.GetName(), d.GetDescription(), d.GetProtocol(), d.GetType())

	var custom func(string) bool
	switch d.GetType() {
	case DatabaseTypeCloudSQL:
		custom = func(val string) bool {
			return strings.EqualFold(val, "cloud") || strings.EqualFold(val, "cloud sql")
		}
	}

	return MatchSearch(fieldVals, values, custom)
}

// setStaticFields sets static resource header and metadata fields.
func (d *DatabaseV3) setStaticFields() {
	d.Kind = KindDatabase
	d.Version = V3
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (d *DatabaseV3) CheckAndSetDefaults() error {
	d.setStaticFields()
	if err := d.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	for key := range d.Spec.DynamicLabels {
		if !IsValidLabelKey(key) {
			return trace.BadParameter("database %q invalid label key: %q", d.GetName(), key)
		}
	}
	if d.Spec.Protocol == "" {
		return trace.BadParameter("database %q protocol is empty", d.GetName())
	}
	if d.Spec.URI == "" {
		return trace.BadParameter("database %q URI is empty", d.GetName())
	}
	// In case of RDS, Aurora or Redshift, AWS information such as region or
	// cluster ID can be extracted from the endpoint if not provided.
	switch {
	case strings.Contains(d.Spec.URI, rdsEndpointSuffix):
		instanceID, region, err := parseRDSEndpoint(d.Spec.URI)
		if err != nil {
			return trace.Wrap(err)
		}
		if d.Spec.AWS.RDS.InstanceID == "" {
			d.Spec.AWS.RDS.InstanceID = instanceID
		}
		if d.Spec.AWS.Region == "" {
			d.Spec.AWS.Region = region
		}
	case strings.Contains(d.Spec.URI, redshiftEndpointSuffix):
		clusterID, region, err := parseRedshiftEndpoint(d.Spec.URI)
		if err != nil {
			return trace.Wrap(err)
		}
		if d.Spec.AWS.Redshift.ClusterID == "" {
			d.Spec.AWS.Redshift.ClusterID = clusterID
		}
		if d.Spec.AWS.Region == "" {
			d.Spec.AWS.Region = region
		}
	case strings.Contains(d.Spec.URI, azureEndpointSuffix):
		name, err := parseAzureEndpoint(d.Spec.URI)
		if err != nil {
			return trace.Wrap(err)
		}
		if d.Spec.Azure.Name == "" {
			d.Spec.Azure.Name = name
		}
	}
	return nil
}

// parseRDSEndpoint extracts region from the provided RDS endpoint.
func parseRDSEndpoint(endpoint string) (instanceID, region string, err error) {
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	// RDS/Aurora endpoint looks like this:
	// aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com
	parts := strings.Split(host, ".")
	if !strings.HasSuffix(host, rdsEndpointSuffix) || len(parts) != 6 {
		return "", "", trace.BadParameter("failed to parse %v as RDS endpoint", endpoint)
	}
	return parts[0], parts[2], nil
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

// parseAzureEndpoint extracts database server name from Azure endpoint.
func parseAzureEndpoint(endpoint string) (name string, err error) {
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", trace.Wrap(err)
	}
	// Azure endpoint looks like this:
	// name.mysql.database.azure.com
	parts := strings.Split(host, ".")
	if !strings.HasSuffix(host, azureEndpointSuffix) || len(parts) != 5 {
		return "", trace.BadParameter("failed to parse %v as Azure endpoint", endpoint)
	}
	return parts[0], nil
}

// GetIAMPolicy returns AWS IAM policy for this database.
func (d *DatabaseV3) GetIAMPolicy() string {
	if d.IsRDS() {
		return d.getRDSPolicy()
	} else if d.IsRedshift() {
		return d.getRedshiftPolicy()
	}
	return ""
}

// GetIAMAction returns AWS IAM action needed to connect to the database.
func (d *DatabaseV3) GetIAMAction() string {
	if d.IsRDS() {
		return "rds-db:connect"
	} else if d.IsRedshift() {
		return "redshift:GetClusterCredentials"
	}
	return ""
}

// GetIAMResources returns AWS IAM resources that provide access to the database.
func (d *DatabaseV3) GetIAMResources() []string {
	aws := d.GetAWS()
	if d.IsRDS() {
		return []string{
			fmt.Sprintf("arn:aws:rds-db:%v:%v:dbuser:%v/*",
				aws.Region, aws.AccountID, aws.RDS.ResourceID),
		}
	} else if d.IsRedshift() {
		return []string{
			fmt.Sprintf("arn:aws:redshift:%v:%v:dbuser:%v/*",
				aws.Region, aws.AccountID, aws.Redshift.ClusterID),
			fmt.Sprintf("arn:aws:redshift:%v:%v:dbname:%v/*",
				aws.Region, aws.AccountID, aws.Redshift.ClusterID),
			fmt.Sprintf("arn:aws:redshift:%v:%v:dbgroup:%v/*",
				aws.Region, aws.AccountID, aws.Redshift.ClusterID),
		}
	}
	return nil
}

// getRDSPolicy returns IAM policy document for this RDS database.
func (d *DatabaseV3) getRDSPolicy() string {
	region := d.GetAWS().Region
	if region == "" {
		region = "<region>"
	}
	accountID := d.GetAWS().AccountID
	if accountID == "" {
		accountID = "<account_id>"
	}
	resourceID := d.GetAWS().RDS.ResourceID
	if resourceID == "" {
		resourceID = "<resource_id>"
	}
	return fmt.Sprintf(rdsPolicyTemplate,
		region, accountID, resourceID)
}

// getRedshiftPolicy returns IAM policy document for this Redshift database.
func (d *DatabaseV3) getRedshiftPolicy() string {
	region := d.GetAWS().Region
	if region == "" {
		region = "<region>"
	}
	accountID := d.GetAWS().AccountID
	if accountID == "" {
		accountID = "<account_id>"
	}
	clusterID := d.GetAWS().Redshift.ClusterID
	if clusterID == "" {
		clusterID = "<cluster_id>"
	}
	return fmt.Sprintf(redshiftPolicyTemplate,
		region, accountID, clusterID)
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
	// DatabaseTypeAzure is Azure-hosted database.
	DatabaseTypeAzure = "azure"
)

// DeduplicateDatabases deduplicates databases by name.
func DeduplicateDatabases(databases []Database) (result []Database) {
	seen := make(map[string]struct{})
	for _, database := range databases {
		if _, ok := seen[database.GetName()]; ok {
			continue
		}
		seen[database.GetName()] = struct{}{}
		result = append(result, database)
	}
	return result
}

// Databases is a list of database resources.
type Databases []Database

// Find returns database with the specified name or nil.
func (d Databases) Find(name string) Database {
	for _, database := range d {
		if database.GetName() == name {
			return database
		}
	}
	return nil
}

// ToMap returns these databases as a map keyed by database name.
func (d Databases) ToMap() map[string]Database {
	m := make(map[string]Database)
	for _, database := range d {
		m[database.GetName()] = database
	}
	return m
}

// AsResources returns these databases as resources with labels.
func (d Databases) AsResources() (resources ResourcesWithLabels) {
	for _, database := range d {
		resources = append(resources, database)
	}
	return resources
}

// Len returns the slice length.
func (d Databases) Len() int { return len(d) }

// Less compares databases by name.
func (d Databases) Less(i, j int) bool { return d[i].GetName() < d[j].GetName() }

// Swap swaps two databases.
func (d Databases) Swap(i, j int) { d[i], d[j] = d[j], d[i] }

const (
	// rdsEndpointSuffix is the RDS/Aurora endpoint suffix.
	rdsEndpointSuffix = ".rds.amazonaws.com"
	// redshiftEndpointSuffix is the Redshift endpoint suffix.
	redshiftEndpointSuffix = ".redshift.amazonaws.com"
	// azureEndpointSuffix is the Azure database endpoint suffix.
	azureEndpointSuffix = ".database.azure.com"
)

var (
	// rdsPolicyTemplate is the IAM policy template for RDS databases access.
	rdsPolicyTemplate = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "rds-db:connect",
      "Resource": "arn:aws:rds-db:%v:%v:dbuser:%v/*"
    }
  ]
}`
	// redshiftPolicyTemplate is the IAM policy template for Redshift databases access.
	redshiftPolicyTemplate = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "redshift:GetClusterCredentials",
      "Resource": [
        "arn:aws:redshift:%[1]v:%[2]v:dbuser:%[3]v/*",
        "arn:aws:redshift:%[1]v:%[2]v:dbname:%[3]v/*",
        "arn:aws:redshift:%[1]v:%[2]v:dbgroup:%[3]v/*"
      ]
    }
  ]
}`
)
