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
	"strings"
	"text/template"
	"time"

	"github.com/gravitational/teleport/api/utils"
	awsutils "github.com/gravitational/teleport/api/utils/aws"
	azureutils "github.com/gravitational/teleport/api/utils/azure"

	"github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
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
	// GetMySQL returns the database options from spec.
	GetMySQL() MySQLOptions
	// GetMySQLServerVersion returns the MySQL server version either from configuration or
	// reported by the database.
	GetMySQLServerVersion() string
	// SetMySQLServerVersion sets the runtime MySQL server version.
	SetMySQLServerVersion(version string)
	// GetAWS returns the database AWS metadata.
	GetAWS() AWS
	// SetStatusAWS sets the database AWS metadata in the status field.
	SetStatusAWS(AWS)
	// GetGCP returns GCP information for Cloud SQL databases.
	GetGCP() GCPCloudSQL
	// GetAzure returns Azure database server metadata.
	GetAzure() Azure
	// SetStatusAzure sets the database Azure metadata in the status field.
	SetStatusAzure(Azure)
	// GetAD returns Active Directory database configuration.
	GetAD() AD
	// GetType returns the database authentication type: self-hosted, RDS, Redshift or Cloud SQL.
	GetType() string
	// GetIAMPolicy returns AWS IAM policy for the database.
	GetIAMPolicy() (string, error)
	// GetIAMAction returns AWS IAM action needed to connect to the database.
	GetIAMAction() string
	// GetIAMResources returns AWS IAM resources that provide access to the database.
	GetIAMResources() []string
	// GetSecretStore returns secret store configurations.
	GetSecretStore() SecretStore
	// GetManagedUsers returns a list of database users that are managed by Teleport.
	GetManagedUsers() []string
	// SetManagedUsers sets a list of database users that are managed by Teleport.
	SetManagedUsers(users []string)
	// IsRDS returns true if this is an RDS/Aurora database.
	IsRDS() bool
	// IsRedshift returns true if this is a Redshift database.
	IsRedshift() bool
	// IsCloudSQL returns true if this is a Cloud SQL database.
	IsCloudSQL() bool
	// IsAzure returns true if this is an Azure database.
	IsAzure() bool
	// IsElastiCache returns true if this is an AWS ElastiCache database.
	IsElastiCache() bool
	// IsMemoryDB returns true if this is an AWS MemoryDB database.
	IsMemoryDB() bool
	// IsAWSHosted returns true if database is hosted by AWS.
	IsAWSHosted() bool
	// IsCloudHosted returns true if database is hosted in the cloud (AWS, Azure or Cloud SQL).
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

// GetMySQL returns the MySQL options from spec.
func (d *DatabaseV3) GetMySQL() MySQLOptions {
	return d.Spec.MySQL
}

// GetMySQLServerVersion returns the MySQL server version reported by the database or the value from configuration
// if the first one is not available.
func (d *DatabaseV3) GetMySQLServerVersion() string {
	if d.Status.MySQL.ServerVersion != "" {
		return d.Status.MySQL.ServerVersion
	}

	return d.Spec.MySQL.ServerVersion
}

// SetMySQLServerVersion sets the runtime MySQL server version.
func (d *DatabaseV3) SetMySQLServerVersion(version string) {
	d.Status.MySQL.ServerVersion = version
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

// IsEmpty returns true if Azure metadata is empty.
func (a Azure) IsEmpty() bool {
	return cmp.Equal(a, Azure{})
}

// GetAzure returns Azure database server metadata.
func (d *DatabaseV3) GetAzure() Azure {
	if !d.Status.Azure.IsEmpty() {
		return d.Status.Azure
	}
	return d.Spec.Azure
}

// SetStatusAzure sets the database Azure metadata in the status field.
func (d *DatabaseV3) SetStatusAzure(azure Azure) {
	d.Status.Azure = azure
}

// GetAD returns Active Directory database configuration.
func (d *DatabaseV3) GetAD() AD {
	return d.Spec.AD
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

// IsElastiCache returns true if this is an AWS ElastiCache database.
func (d *DatabaseV3) IsElastiCache() bool {
	return d.GetType() == DatabaseTypeElastiCache
}

// IsMemoryDB returns true if this is an AWS MemoryDB database.
func (d *DatabaseV3) IsMemoryDB() bool {
	return d.GetType() == DatabaseTypeMemoryDB
}

// IsAWSHosted returns true if database is hosted by AWS.
func (d *DatabaseV3) IsAWSHosted() bool {
	return d.IsRDS() || d.IsRedshift() || d.IsElastiCache() || d.IsMemoryDB()
}

// IsCloudHosted returns true if database is hosted in the cloud (AWS, Azure or
// Cloud SQL).
func (d *DatabaseV3) IsCloudHosted() bool {
	return d.IsAWSHosted() || d.IsCloudSQL() || d.IsAzure()
}

// GetType returns the database type.
func (d *DatabaseV3) GetType() string {
	if d.GetAWS().Redshift.ClusterID != "" {
		return DatabaseTypeRedshift
	}
	if d.GetAWS().ElastiCache.ReplicationGroupID != "" {
		return DatabaseTypeElastiCache
	}
	if d.GetAWS().MemoryDB.ClusterName != "" {
		return DatabaseTypeMemoryDB
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
	if d.Spec.MySQL.ServerVersion != "" && d.Spec.Protocol != "mysql" {
		return trace.BadParameter("MySQL ServerVersion can be only set for MySQL database")
	}
	// In case of RDS, Aurora or Redshift, AWS information such as region or
	// cluster ID can be extracted from the endpoint if not provided.
	switch {
	case awsutils.IsRDSEndpoint(d.Spec.URI):
		instanceID, region, err := awsutils.ParseRDSEndpoint(d.Spec.URI)
		if err != nil {
			return trace.Wrap(err)
		}
		if d.Spec.AWS.RDS.InstanceID == "" {
			d.Spec.AWS.RDS.InstanceID = instanceID
		}
		if d.Spec.AWS.Region == "" {
			d.Spec.AWS.Region = region
		}
	case awsutils.IsRedshiftEndpoint(d.Spec.URI):
		clusterID, region, err := awsutils.ParseRedshiftEndpoint(d.Spec.URI)
		if err != nil {
			return trace.Wrap(err)
		}
		if d.Spec.AWS.Redshift.ClusterID == "" {
			d.Spec.AWS.Redshift.ClusterID = clusterID
		}
		if d.Spec.AWS.Region == "" {
			d.Spec.AWS.Region = region
		}
	case awsutils.IsElastiCacheEndpoint(d.Spec.URI):
		endpointInfo, err := awsutils.ParseElastiCacheEndpoint(d.Spec.URI)
		if err != nil {
			logrus.WithError(err).Warnf("Failed to parse %v as ElastiCache endpoint", d.Spec.URI)
			break
		}
		if d.Spec.AWS.ElastiCache.ReplicationGroupID == "" {
			d.Spec.AWS.ElastiCache.ReplicationGroupID = endpointInfo.ID
		}
		if d.Spec.AWS.Region == "" {
			d.Spec.AWS.Region = endpointInfo.Region
		}
		d.Spec.AWS.ElastiCache.TransitEncryptionEnabled = endpointInfo.TransitEncryptionEnabled
		d.Spec.AWS.ElastiCache.EndpointType = endpointInfo.EndpointType
	case awsutils.IsMemoryDBEndpoint(d.Spec.URI):
		endpointInfo, err := awsutils.ParseMemoryDBEndpoint(d.Spec.URI)
		if err != nil {
			logrus.WithError(err).Warnf("Failed to parse %v as MemoryDB endpoint", d.Spec.URI)
			break
		}
		if d.Spec.AWS.MemoryDB.ClusterName == "" {
			d.Spec.AWS.MemoryDB.ClusterName = endpointInfo.ID
		}
		if d.Spec.AWS.Region == "" {
			d.Spec.AWS.Region = endpointInfo.Region
		}
		d.Spec.AWS.MemoryDB.TLSEnabled = endpointInfo.TransitEncryptionEnabled
		d.Spec.AWS.MemoryDB.EndpointType = endpointInfo.EndpointType

	case azureutils.IsDatabaseEndpoint(d.Spec.URI):
		// For Azure MySQL and PostgresSQL.
		name, err := azureutils.ParseDatabaseEndpoint(d.Spec.URI)
		if err != nil {
			return trace.Wrap(err)
		}
		if d.Spec.Azure.Name == "" {
			d.Spec.Azure.Name = name
		}
	case azureutils.IsCacheForRedisEndpoint(d.Spec.URI):
		// ResourceID is required for fetching Redis tokens.
		if d.Spec.Azure.ResourceID == "" {
			return trace.BadParameter("missing ResourceID for Azure Cache %v", d.Metadata.Name)
		}

		name, err := azureutils.ParseCacheForRedisEndpoint(d.Spec.URI)
		if err != nil {
			return trace.Wrap(err)
		}

		if d.Spec.Azure.Name == "" {
			d.Spec.Azure.Name = name
		}
	}
	return nil
}

// GetIAMPolicy returns AWS IAM policy for this database.
func (d *DatabaseV3) GetIAMPolicy() (string, error) {
	if d.IsRDS() {
		policy, err := d.getRDSPolicy()
		return policy, trace.Wrap(err)
	} else if d.IsRedshift() {
		policy, err := d.getRedshiftPolicy()
		return policy, trace.Wrap(err)
	}
	return "", trace.BadParameter("GetIAMPolicy is not supported policy for database type %s", d.GetType())
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
	partition := awsutils.GetPartitionFromRegion(aws.Region)
	if d.IsRDS() {
		if aws.Region != "" && aws.AccountID != "" && aws.RDS.ResourceID != "" {
			return []string{
				fmt.Sprintf("arn:%v:rds-db:%v:%v:dbuser:%v/*",
					partition, aws.Region, aws.AccountID, aws.RDS.ResourceID),
			}
		}
	} else if d.IsRedshift() {
		if aws.Region != "" && aws.AccountID != "" && aws.Redshift.ClusterID != "" {
			return []string{
				fmt.Sprintf("arn:%v:redshift:%v:%v:dbuser:%v/*",
					partition, aws.Region, aws.AccountID, aws.Redshift.ClusterID),
				fmt.Sprintf("arn:%v:redshift:%v:%v:dbname:%v/*",
					partition, aws.Region, aws.AccountID, aws.Redshift.ClusterID),
				fmt.Sprintf("arn:%v:redshift:%v:%v:dbgroup:%v/*",
					partition, aws.Region, aws.AccountID, aws.Redshift.ClusterID),
			}
		}
	}
	return nil
}

// GetSecretStore returns secret store configurations.
func (d *DatabaseV3) GetSecretStore() SecretStore {
	return d.Spec.AWS.SecretStore
}

// GetManagedUsers returns a list of database users that are managed by Teleport.
func (d *DatabaseV3) GetManagedUsers() []string {
	return d.Status.ManagedUsers
}

// SetManagedUsers sets a list of database users that are managed by Teleport.
func (d *DatabaseV3) SetManagedUsers(users []string) {
	d.Status.ManagedUsers = users
}

// getRDSPolicy returns IAM policy document for this RDS database.
func (d *DatabaseV3) getRDSPolicy() (string, error) {
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

	var sb strings.Builder
	err := rdsPolicyTemplate.Execute(&sb, arnTemplateInput{
		Partition:  awsutils.GetPartitionFromRegion(region),
		Region:     region,
		AccountID:  accountID,
		ResourceID: resourceID,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return sb.String(), nil
}

// getRedshiftPolicy returns IAM policy document for this Redshift database.
func (d *DatabaseV3) getRedshiftPolicy() (string, error) {
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

	var sb strings.Builder
	err := redshiftPolicyTemplate.Execute(&sb, arnTemplateInput{
		Partition:  awsutils.GetPartitionFromRegion(region),
		Region:     region,
		AccountID:  accountID,
		ResourceID: clusterID,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return sb.String(), nil
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
	// DatabaseTypeElastiCache is AWS-hosted ElastiCache database.
	DatabaseTypeElastiCache = "elasticache"
	// DatabaseTypeMemoryDB is AWS-hosted MemoryDB database.
	DatabaseTypeMemoryDB = "memorydb"
)

// GetServerName returns the GCP database project and instance as "<project-id>:<instance-id>".
func (gcp GCPCloudSQL) GetServerName() string {
	return fmt.Sprintf("%s:%s", gcp.ProjectID, gcp.InstanceID)
}

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

type arnTemplateInput struct {
	Partition, Region, AccountID, ResourceID string
}

var (
	// rdsPolicyTemplate is the IAM policy template for RDS databases access.
	rdsPolicyTemplate = template.Must(template.New("").Parse(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "rds-db:connect",
      "Resource": "arn:{{.Partition}}:rds-db:{{.Region}}:{{.AccountID}}:dbuser:{{.ResourceID}}/*"
    }
  ]
}`))
	// redshiftPolicyTemplate is the IAM policy template for Redshift databases access.
	redshiftPolicyTemplate = template.Must(template.New("").Parse(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "redshift:GetClusterCredentials",
      "Resource": [
        "arn:{{.Partition}}:redshift:{{.Region}}:{{.AccountID}}:dbuser:{{.ResourceID}}/*",
        "arn:{{.Partition}}:redshift:{{.Region}}:{{.AccountID}}:dbname:{{.ResourceID}}/*",
        "arn:{{.Partition}}:redshift:{{.Region}}:{{.AccountID}}:dbgroup:{{.ResourceID}}/*"
      ]
    }
  ]
}`))
)
