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
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/utils"
	atlasutils "github.com/gravitational/teleport/api/utils/atlas"
	awsutils "github.com/gravitational/teleport/api/utils/aws"
	azureutils "github.com/gravitational/teleport/api/utils/azure"
)

// Database represents a single database proxied by a database server.
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
	// SetCA sets the database CA certificate in the Spec.TLS field.
	SetCA(string)
	// GetTLS returns the database TLS configuration.
	GetTLS() DatabaseTLS
	// SetStatusCA sets the database CA certificate in the status field.
	SetStatusCA(string)
	// GetStatusCA gets the database CA certificate in the status field.
	GetStatusCA() string
	// GetMySQL returns the database options from spec.
	GetMySQL() MySQLOptions
	// GetOracle returns the database options from spec.
	GetOracle() OracleOptions
	// GetMySQLServerVersion returns the MySQL server version either from configuration or
	// reported by the database.
	GetMySQLServerVersion() string
	// SetMySQLServerVersion sets the runtime MySQL server version.
	SetMySQLServerVersion(version string)
	// GetAWS returns the database AWS metadata.
	GetAWS() AWS
	// SetStatusAWS sets the database AWS metadata in the status field.
	SetStatusAWS(AWS)
	// SetAWSExternalID sets the database AWS external ID in the Spec.AWS field.
	SetAWSExternalID(id string)
	// SetAWSAssumeRole sets the database AWS assume role arn in the Spec.AWS field.
	SetAWSAssumeRole(roleARN string)
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
	// GetSecretStore returns secret store configurations.
	GetSecretStore() SecretStore
	// GetManagedUsers returns a list of database users that are managed by Teleport.
	GetManagedUsers() []string
	// SetManagedUsers sets a list of database users that are managed by Teleport.
	SetManagedUsers(users []string)
	// GetMongoAtlas returns Mongo Atlas database metadata.
	GetMongoAtlas() MongoAtlas
	// IsRDS returns true if this is an RDS/Aurora database.
	IsRDS() bool
	// IsRDSProxy returns true if this is an RDS Proxy database.
	IsRDSProxy() bool
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
	// RequireAWSIAMRolesAsUsers returns true for database types that require
	// AWS IAM roles as database users.
	RequireAWSIAMRolesAsUsers() bool
	// SupportAWSIAMRoleARNAsUsers returns true for database types that support
	// AWS IAM roles as database users.
	SupportAWSIAMRoleARNAsUsers() bool
	// Copy returns a copy of this database resource.
	Copy() *DatabaseV3
	// GetAdminUser returns database privileged user information.
	GetAdminUser() DatabaseAdminUser
	// SupportsAutoUsers returns true if this database supports automatic
	// user provisioning.
	SupportsAutoUsers() bool
	// GetEndpointType returns the endpoint type of the database, if available.
	GetEndpointType() string
	// GetCloud gets the cloud this database is running on, or an empty string if it
	// isn't running on a cloud provider.
	GetCloud() string
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

// GetRevision returns the revision
func (d *DatabaseV3) GetRevision() string {
	return d.Metadata.GetRevision()
}

// SetRevision sets the revision
func (d *DatabaseV3) SetRevision(rev string) {
	d.Metadata.SetRevision(rev)
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

// GetLabel retrieves the label with the provided key. If not found
// value will be empty and ok will be false.
func (d *DatabaseV3) GetLabel(key string) (value string, ok bool) {
	if cmd, ok := d.Spec.DynamicLabels[key]; ok {
		return cmd.Result, ok
	}

	v, ok := d.Metadata.Labels[key]
	return v, ok
}

// GetAllLabels returns the database combined static and dynamic labels.
func (d *DatabaseV3) GetAllLabels() map[string]string {
	return CombineLabels(d.Metadata.Labels, d.Spec.DynamicLabels)
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

// GetAdminUser returns database privileged user information.
func (d *DatabaseV3) GetAdminUser() (ret DatabaseAdminUser) {
	// First check the spec.
	if d.Spec.AdminUser != nil {
		ret = *d.Spec.AdminUser
	}

	// If it's not in the spec, check labels (for auto-discovered databases).
	// TODO Azure will require different labels.
	if d.Origin() == OriginCloud {
		if ret.Name == "" {
			ret.Name = d.Metadata.Labels[DatabaseAdminLabel]
		}
		if ret.DefaultDatabase == "" {
			ret.DefaultDatabase = d.Metadata.Labels[DatabaseAdminDefaultDatabaseLabel]
		}
	}
	return
}

// GetOracle returns the Oracle options from spec.
func (d *DatabaseV3) GetOracle() OracleOptions {
	return d.Spec.Oracle
}

// SupportsAutoUsers returns true if this database supports automatic user
// provisioning.
func (d *DatabaseV3) SupportsAutoUsers() bool {
	switch d.GetProtocol() {
	case DatabaseProtocolPostgreSQL:
		switch d.GetType() {
		case DatabaseTypeSelfHosted, DatabaseTypeRDS, DatabaseTypeRedshift:
			return true
		}
	case DatabaseProtocolMySQL:
		switch d.GetType() {
		case DatabaseTypeSelfHosted, DatabaseTypeRDS:
			return true
		}

	case DatabaseProtocolMongoDB:
		switch d.GetType() {
		case DatabaseTypeSelfHosted:
			return true
		}
	}
	return false
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

// SetCA sets the database CA certificate in the Spec.TLS.CACert field.
func (d *DatabaseV3) SetCA(caCert string) {
	d.Spec.TLS.CACert = caCert
}

// GetTLS returns Database TLS configuration.
func (d *DatabaseV3) GetTLS() DatabaseTLS {
	return d.Spec.TLS
}

// SetStatusCA sets the database CA certificate in the status field.
func (d *DatabaseV3) SetStatusCA(ca string) {
	d.Status.CACert = ca
}

// GetStatusCA gets the database CA certificate in the status field.
func (d *DatabaseV3) GetStatusCA() string {
	return d.Status.CACert
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
	return protoKnownFieldsEqual(&a, &AWS{})
}

// Partition returns the AWS partition based on the region.
func (a AWS) Partition() string {
	return awsutils.GetPartitionFromRegion(a.Region)
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

// SetAWSExternalID sets the database AWS external ID in the Spec.AWS field.
func (d *DatabaseV3) SetAWSExternalID(id string) {
	d.Spec.AWS.ExternalID = id
}

// SetAWSAssumeRole sets the database AWS assume role arn in the Spec.AWS field.
func (d *DatabaseV3) SetAWSAssumeRole(roleARN string) {
	d.Spec.AWS.AssumeRoleARN = roleARN
}

// GetGCP returns GCP information for Cloud SQL databases.
func (d *DatabaseV3) GetGCP() GCPCloudSQL {
	return d.Spec.GCP
}

// IsEmpty returns true if Azure metadata is empty.
func (a Azure) IsEmpty() bool {
	return protoKnownFieldsEqual(&a, &Azure{})
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

// IsRDSProxy returns true if this is an AWS RDS Proxy database.
func (d *DatabaseV3) IsRDSProxy() bool {
	return d.GetType() == DatabaseTypeRDSProxy
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

// IsAWSKeyspaces returns true if this is an AWS hosted Cassandra database.
func (d *DatabaseV3) IsAWSKeyspaces() bool {
	return d.GetType() == DatabaseTypeAWSKeyspaces
}

// IsDynamoDB returns true if this is an AWS hosted DynamoDB database.
func (d *DatabaseV3) IsDynamoDB() bool {
	return d.GetType() == DatabaseTypeDynamoDB
}

// IsOpenSearch returns true if this is an AWS hosted OpenSearch instance.
func (d *DatabaseV3) IsOpenSearch() bool {
	return d.GetType() == DatabaseTypeOpenSearch
}

// IsAWSHosted returns true if database is hosted by AWS.
func (d *DatabaseV3) IsAWSHosted() bool {
	_, ok := d.getAWSType()
	return ok
}

// IsCloudHosted returns true if database is hosted in the cloud (AWS, Azure or
// Cloud SQL).
func (d *DatabaseV3) IsCloudHosted() bool {
	return d.IsAWSHosted() || d.IsCloudSQL() || d.IsAzure()
}

// GetCloud gets the cloud this database is running on, or an empty string if it
// isn't running on a cloud provider.
func (d *DatabaseV3) GetCloud() string {
	switch {
	case d.IsAWSHosted():
		return CloudAWS
	case d.IsCloudSQL():
		return CloudGCP
	case d.IsAzure():
		return CloudAzure
	default:
		return ""
	}
}

// getAWSType returns the database type.
func (d *DatabaseV3) getAWSType() (string, bool) {
	aws := d.GetAWS()
	switch d.Spec.Protocol {
	case DatabaseTypeCassandra:
		if !aws.IsEmpty() {
			return DatabaseTypeAWSKeyspaces, true
		}
	case DatabaseTypeDynamoDB:
		return DatabaseTypeDynamoDB, true
	case DatabaseTypeOpenSearch:
		return DatabaseTypeOpenSearch, true
	}
	if aws.Redshift.ClusterID != "" {
		return DatabaseTypeRedshift, true
	}
	if aws.RedshiftServerless.WorkgroupName != "" || aws.RedshiftServerless.EndpointName != "" {
		return DatabaseTypeRedshiftServerless, true
	}
	if aws.ElastiCache.ReplicationGroupID != "" {
		return DatabaseTypeElastiCache, true
	}
	if aws.MemoryDB.ClusterName != "" {
		return DatabaseTypeMemoryDB, true
	}
	if aws.RDSProxy.Name != "" || aws.RDSProxy.CustomEndpointName != "" {
		return DatabaseTypeRDSProxy, true
	}
	if aws.Region != "" || aws.RDS.InstanceID != "" || aws.RDS.ResourceID != "" || aws.RDS.ClusterID != "" {
		return DatabaseTypeRDS, true
	}
	return "", false
}

// GetType returns the database type.
func (d *DatabaseV3) GetType() string {
	if d.GetMongoAtlas().Name != "" {
		return DatabaseTypeMongoAtlas
	}

	if awsType, ok := d.getAWSType(); ok {
		return awsType
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
	return utils.CloneProtoMsg(d)
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

// validDatabaseNameRegexp filters the allowed characters in database names.
// This is the (almost) the same regexp used to check for valid DNS 1035 labels,
// except we allow uppercase chars.
var validDatabaseNameRegexp = regexp.MustCompile(`^[a-zA-Z]([-a-zA-Z0-9]*[a-zA-Z0-9])?$`)

// ValidateDatabaseName returns an error if a given string is not a valid
// Database name.
// Unlike application access proxy, database name doesn't necessarily
// need to be a valid subdomain but use the same validation logic for the
// simplicity and consistency, except two differences: don't restrict names to
// 63 chars in length and allow upper case chars.
func ValidateDatabaseName(name string) error {
	return ValidateResourceName(validDatabaseNameRegexp, name)
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (d *DatabaseV3) CheckAndSetDefaults() error {
	d.setStaticFields()
	if err := d.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if err := ValidateDatabaseName(d.GetName()); err != nil {
		return trace.Wrap(err, "invalid database name")
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
		switch d.GetType() {
		case DatabaseTypeAWSKeyspaces:
			if d.Spec.AWS.Region != "" {
				// In case of AWS Hosted Cassandra allow to omit URI.
				// The URL will be constructed from the database resource based on the region and account ID.
				d.Spec.URI = awsutils.CassandraEndpointURLForRegion(d.Spec.AWS.Region)
			} else {
				return trace.BadParameter("AWS Keyspaces database %q URI is empty and cannot be derived without a configured AWS region",
					d.GetName())
			}
		case DatabaseTypeDynamoDB:
			if d.Spec.AWS.Region != "" {
				d.Spec.URI = awsutils.DynamoDBURIForRegion(d.Spec.AWS.Region)
			} else {
				return trace.BadParameter("DynamoDB database %q URI is empty and cannot be derived without a configured AWS region",
					d.GetName())
			}
		default:
			return trace.BadParameter("database %q URI is empty", d.GetName())
		}
	}
	if d.Spec.MySQL.ServerVersion != "" && d.Spec.Protocol != "mysql" {
		return trace.BadParameter("database %q MySQL ServerVersion can be only set for MySQL database",
			d.GetName())
	}

	// In case of RDS, Aurora or Redshift, AWS information such as region or
	// cluster ID can be extracted from the endpoint if not provided.
	switch {
	case d.IsDynamoDB():
		if err := d.handleDynamoDBConfig(); err != nil {
			return trace.Wrap(err)
		}
	case d.IsOpenSearch():
		if err := d.handleOpenSearchConfig(); err != nil {
			return trace.Wrap(err)
		}
	case awsutils.IsRDSEndpoint(d.Spec.URI):
		details, err := awsutils.ParseRDSEndpoint(d.Spec.URI)
		if err != nil {
			logrus.WithError(err).Warnf("Failed to parse RDS endpoint %v.", d.Spec.URI)
			break
		}
		if d.Spec.AWS.RDS.InstanceID == "" {
			d.Spec.AWS.RDS.InstanceID = details.InstanceID
		}
		if d.Spec.AWS.RDS.ClusterID == "" {
			d.Spec.AWS.RDS.ClusterID = details.ClusterID
		}
		if d.Spec.AWS.RDSProxy.Name == "" {
			d.Spec.AWS.RDSProxy.Name = details.ProxyName
		}
		if d.Spec.AWS.RDSProxy.CustomEndpointName == "" {
			d.Spec.AWS.RDSProxy.CustomEndpointName = details.ProxyCustomEndpointName
		}
		if d.Spec.AWS.Region == "" {
			d.Spec.AWS.Region = details.Region
		}
		if details.ClusterCustomEndpointName != "" && d.Spec.AWS.RDS.ClusterID == "" {
			return trace.BadParameter("database %q missing RDS ClusterID for RDS Aurora custom endpoint %v",
				d.GetName(), d.Spec.URI)
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
	case awsutils.IsRedshiftServerlessEndpoint(d.Spec.URI):
		details, err := awsutils.ParseRedshiftServerlessEndpoint(d.Spec.URI)
		if err != nil {
			logrus.WithError(err).Warnf("Failed to parse Redshift Serverless endpoint %v.", d.Spec.URI)
			break
		}
		if d.Spec.AWS.RedshiftServerless.WorkgroupName == "" {
			d.Spec.AWS.RedshiftServerless.WorkgroupName = details.WorkgroupName
		}
		if d.Spec.AWS.RedshiftServerless.EndpointName == "" {
			d.Spec.AWS.RedshiftServerless.EndpointName = details.EndpointName
		}
		if d.Spec.AWS.AccountID == "" {
			d.Spec.AWS.AccountID = details.AccountID
		}
		if d.Spec.AWS.Region == "" {
			d.Spec.AWS.Region = details.Region
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
	case awsutils.IsKeyspacesEndpoint(d.Spec.URI):
		if d.Spec.AWS.AccountID == "" {
			return trace.BadParameter("database %q AWS account ID is empty",
				d.GetName())
		}
		if d.Spec.AWS.Region == "" {
			switch {
			case d.IsAWSKeyspaces():
				region, err := awsutils.CassandraEndpointRegion(d.Spec.URI)
				if err != nil {
					return trace.Wrap(err)
				}
				d.Spec.AWS.Region = region
			default:
				return trace.BadParameter("database %q AWS region is empty",
					d.GetName())
			}
		}
	case azureutils.IsCacheForRedisEndpoint(d.Spec.URI):
		// ResourceID is required for fetching Redis tokens.
		if d.Spec.Azure.ResourceID == "" {
			return trace.BadParameter("database %q Azure resource ID is empty",
				d.GetName())
		}

		name, err := azureutils.ParseCacheForRedisEndpoint(d.Spec.URI)
		if err != nil {
			return trace.Wrap(err)
		}

		if d.Spec.Azure.Name == "" {
			d.Spec.Azure.Name = name
		}
	case azureutils.IsMSSQLServerEndpoint(d.Spec.URI):
		if d.Spec.Azure.Name == "" {
			name, err := azureutils.ParseMSSQLEndpoint(d.Spec.URI)
			if err != nil {
				return trace.Wrap(err)
			}
			d.Spec.Azure.Name = name
		}
	case atlasutils.IsAtlasEndpoint(d.Spec.URI):
		name, err := atlasutils.ParseAtlasEndpoint(d.Spec.URI)
		if err != nil {
			return trace.Wrap(err)
		}
		d.Spec.MongoAtlas.Name = name
	}

	// Validate AWS Specific configuration
	if d.Spec.AWS.AccountID != "" {
		if err := awsutils.IsValidAccountID(d.Spec.AWS.AccountID); err != nil {
			return trace.BadParameter("database %q has invalid AWS account ID: %v",
				d.GetName(), err)
		}
	}

	if d.Spec.AWS.ExternalID != "" && d.Spec.AWS.AssumeRoleARN == "" && !d.RequireAWSIAMRolesAsUsers() {
		// Databases that use database username to assume an IAM role do not
		// need assume_role_arn in configuration when external_id is set.
		return trace.BadParameter("AWS database %q has external_id %q, but assume_role_arn is empty",
			d.GetName(), d.Spec.AWS.ExternalID)
	}

	// Validate Cloud SQL specific configuration.
	switch {
	case d.Spec.GCP.ProjectID != "" && d.Spec.GCP.InstanceID == "":
		return trace.BadParameter("database %q missing Cloud SQL instance ID",
			d.GetName())
	case d.Spec.GCP.ProjectID == "" && d.Spec.GCP.InstanceID != "":
		return trace.BadParameter("database %q missing Cloud SQL project ID",
			d.GetName())
	}

	// Admin user should only be specified for databases that support automatic
	// user provisioning.
	if d.GetAdminUser().Name != "" && !d.SupportsAutoUsers() {
		return trace.BadParameter("cannot set admin user on database %q: %v/%v databases don't support automatic user provisioning yet",
			d.GetName(), d.GetProtocol(), d.GetType())
	}

	switch protocol := d.GetProtocol(); protocol {
	case DatabaseProtocolClickHouseHTTP, DatabaseProtocolClickHouse:
		const (
			clickhouseNativeSchema = "clickhouse"
			clickhouseHTTPSchema   = "https"
		)
		parts := strings.Split(d.GetURI(), ":")
		if len(parts) == 3 {
			break
		} else if len(parts) != 2 {
			return trace.BadParameter("invalid ClickHouse URL %s", d.GetURI())
		}

		if !strings.HasPrefix(d.Spec.URI, clickhouseHTTPSchema) && protocol == DatabaseProtocolClickHouseHTTP {
			d.Spec.URI = fmt.Sprintf("%s://%s", clickhouseHTTPSchema, d.Spec.URI)
		}
		if protocol == DatabaseProtocolClickHouse {
			d.Spec.URI = fmt.Sprintf("%s://%s", clickhouseNativeSchema, d.Spec.URI)
		}
	}

	return nil
}

// handleDynamoDBConfig handles DynamoDB configuration checking.
func (d *DatabaseV3) handleDynamoDBConfig() error {
	if d.Spec.AWS.AccountID == "" {
		return trace.BadParameter("database %q AWS account ID is empty", d.GetName())
	}

	info, err := awsutils.ParseDynamoDBEndpoint(d.Spec.URI)
	switch {
	case err != nil:
		// when region parsing returns an error but the region is set, it's ok because we can just construct the URI using the region,
		// so we check if the region is configured to see if this is really a configuration error.
		if d.Spec.AWS.Region == "" {
			// the AWS region is empty and we can't derive it from the URI, so this is a config error.
			return trace.BadParameter("database %q AWS region is empty and cannot be derived from the URI %q",
				d.GetName(), d.Spec.URI)
		}
		if awsutils.IsAWSEndpoint(d.Spec.URI) {
			// The user configured an AWS URI that doesn't look like a DynamoDB endpoint.
			// The URI must look like <service>.<region>.<partition> or <region>.<partition>
			return trace.Wrap(err)
		}
	case d.Spec.AWS.Region == "":
		// if the AWS region is empty we can just use the region extracted from the URI.
		d.Spec.AWS.Region = info.Region
	case d.Spec.AWS.Region != info.Region:
		// if the AWS region is not empty but doesn't match the URI, this may indicate a user configuration mistake.
		return trace.BadParameter("database %q AWS region %q does not match the configured URI region %q,"+
			" omit the URI and it will be derived automatically for the configured AWS region",
			d.GetName(), d.Spec.AWS.Region, info.Region)
	}

	if d.Spec.URI == "" {
		d.Spec.URI = awsutils.DynamoDBURIForRegion(d.Spec.AWS.Region)
	}
	return nil
}

// handleOpenSearchConfig handles OpenSearch configuration checks.
func (d *DatabaseV3) handleOpenSearchConfig() error {
	if d.Spec.AWS.AccountID == "" {
		return trace.BadParameter("database %q AWS account ID is empty", d.GetName())
	}

	info, err := awsutils.ParseOpensearchEndpoint(d.Spec.URI)
	switch {
	case err != nil:
		// parsing the endpoint can return an error, especially if the custom endpoint feature is in use.
		// this is fine as long as we have the region explicitly configured.
		if d.Spec.AWS.Region == "" {
			// the AWS region is empty, and we can't derive it from the URI, so this is a config error.
			return trace.BadParameter("database %q AWS region is missing and cannot be derived from the URI %q",
				d.GetName(), d.Spec.URI)
		}
		if awsutils.IsAWSEndpoint(d.Spec.URI) {
			// The user configured an AWS URI that doesn't look like a OpenSearch endpoint.
			// The URI must look like: <region>.<service>.<partition>.
			return trace.Wrap(err)
		}
	case d.Spec.AWS.Region == "":
		// if the AWS region is empty we can just use the region extracted from the URI.
		d.Spec.AWS.Region = info.Region
	case d.Spec.AWS.Region != info.Region:
		// if the AWS region is not empty but doesn't match the URI, this may indicate a user configuration mistake.
		return trace.BadParameter("database %q AWS region %q does not match the configured URI region %q,"+
			" omit the URI and it will be derived automatically for the configured AWS region",
			d.GetName(), d.Spec.AWS.Region, info.Region)
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

// GetMongoAtlas returns Mongo Atlas database metadata.
func (d *DatabaseV3) GetMongoAtlas() MongoAtlas {
	return d.Spec.MongoAtlas
}

// RequireAWSIAMRolesAsUsers returns true for database types that require AWS
// IAM roles as database users.
// IMPORTANT: if you add a database that requires AWS IAM Roles as users,
// and that database supports discovery, be sure to update RequireAWSIAMRolesAsUsersMatchers
// in matchers_aws.go as well.
func (d *DatabaseV3) RequireAWSIAMRolesAsUsers() bool {
	awsType, ok := d.getAWSType()
	if !ok {
		return false
	}

	switch awsType {
	case DatabaseTypeAWSKeyspaces,
		DatabaseTypeDynamoDB,
		DatabaseTypeOpenSearch,
		DatabaseTypeRedshiftServerless:
		return true
	default:
		return false
	}
}

// SupportAWSIAMRoleARNAsUsers returns true for database types that support AWS
// IAM roles as database users.
func (d *DatabaseV3) SupportAWSIAMRoleARNAsUsers() bool {
	switch d.GetType() {
	// Note that databases in this list use IAM auth when:
	// - the database user is a full AWS role ARN role
	// - or the database user starts with "role/"
	//
	// Other database users will fallback to default auth methods (e.g X.509 for
	// MongoAtlas, regular auth token for Redshift).
	//
	// Therefore it is important to make sure "/" is an invalid character for
	// regular in-database usernames so that "role/" can be differentiated from
	// regular usernames.
	case DatabaseTypeMongoAtlas,
		DatabaseTypeRedshift:
		return true
	default:
		return false
	}
}

// GetEndpointType returns the endpoint type of the database, if available.
func (d *DatabaseV3) GetEndpointType() string {
	if endpointType, ok := d.GetStaticLabels()[DiscoveryLabelEndpointType]; ok {
		return endpointType
	}
	switch d.GetType() {
	case DatabaseTypeElastiCache:
		return d.GetAWS().ElastiCache.EndpointType
	case DatabaseTypeMemoryDB:
		return d.GetAWS().MemoryDB.EndpointType
	case DatabaseTypeOpenSearch:
		return d.GetAWS().OpenSearch.EndpointType
	case DatabaseTypeRDS:
		// If not available from discovery tags, get the endpoint type from the
		// URL.
		if details, err := awsutils.ParseRDSEndpoint(d.GetURI()); err == nil {
			return details.EndpointType
		}
	}
	return ""
}

const (
	// DatabaseProtocolPostgreSQL is the PostgreSQL database protocol.
	DatabaseProtocolPostgreSQL = "postgres"
	// DatabaseProtocolClickHouseHTTP is the ClickHouse database HTTP protocol.
	DatabaseProtocolClickHouseHTTP = "clickhouse-http"
	// DatabaseProtocolClickHouse is the ClickHouse database native write protocol.
	DatabaseProtocolClickHouse = "clickhouse"
	// DatabaseProtocolMySQL is the MySQL database protocol.
	DatabaseProtocolMySQL = "mysql"
	// DatabaseProtocolMongoDB is the MongoDB database protocol.
	DatabaseProtocolMongoDB = "mongodb"

	// DatabaseTypeSelfHosted is the self-hosted type of database.
	DatabaseTypeSelfHosted = "self-hosted"
	// DatabaseTypeRDS is AWS-hosted RDS or Aurora database.
	DatabaseTypeRDS = "rds"
	// DatabaseTypeRDSProxy is an AWS-hosted RDS Proxy.
	DatabaseTypeRDSProxy = "rdsproxy"
	// DatabaseTypeRedshift is AWS Redshift database.
	DatabaseTypeRedshift = "redshift"
	// DatabaseTypeRedshiftServerless is AWS Redshift Serverless database.
	DatabaseTypeRedshiftServerless = "redshift-serverless"
	// DatabaseTypeCloudSQL is GCP-hosted Cloud SQL database.
	DatabaseTypeCloudSQL = "gcp"
	// DatabaseTypeAzure is Azure-hosted database.
	DatabaseTypeAzure = "azure"
	// DatabaseTypeElastiCache is AWS-hosted ElastiCache database.
	DatabaseTypeElastiCache = "elasticache"
	// DatabaseTypeMemoryDB is AWS-hosted MemoryDB database.
	DatabaseTypeMemoryDB = "memorydb"
	// DatabaseTypeAWSKeyspaces is AWS-hosted Keyspaces database (Cassandra).
	DatabaseTypeAWSKeyspaces = "keyspace"
	// DatabaseTypeCassandra is AWS-hosted Keyspace database.
	DatabaseTypeCassandra = "cassandra"
	// DatabaseTypeDynamoDB is a DynamoDB database.
	DatabaseTypeDynamoDB = "dynamodb"
	// DatabaseTypeOpenSearch is AWS-hosted OpenSearch instance.
	DatabaseTypeOpenSearch = "opensearch"
	// DatabaseTypeMongoAtlas
	DatabaseTypeMongoAtlas = "mongo-atlas"
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

// UnmarshalJSON supports parsing DatabaseTLSMode from number or string.
func (d *DatabaseTLSMode) UnmarshalJSON(data []byte) error {
	type loopBreaker DatabaseTLSMode
	var val loopBreaker
	// try as number first.
	if err := json.Unmarshal(data, &val); err == nil {
		*d = DatabaseTLSMode(val)
		return nil
	}

	// fallback to string.
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return trace.Wrap(err)
	}
	return d.decodeName(s)
}

// UnmarshalYAML supports parsing DatabaseTLSMode from number or string.
func (d *DatabaseTLSMode) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// try as number first.
	type loopBreaker DatabaseTLSMode
	var val loopBreaker
	if err := unmarshal(&val); err == nil {
		*d = DatabaseTLSMode(val)
		return nil
	}

	// fallback to string.
	var s string
	if err := unmarshal(&s); err != nil {
		return trace.Wrap(err)
	}
	return d.decodeName(s)
}

// decodeName decodes DatabaseTLSMode from a string. This is necessary for
// allowing tctl commands to work with the same names as documented in Teleport
// configuration, rather than requiring it be specified as an unreadable enum
// number.
func (d *DatabaseTLSMode) decodeName(name string) error {
	switch name {
	case "verify-full", "":
		*d = DatabaseTLSMode_VERIFY_FULL
		return nil
	case "verify-ca":
		*d = DatabaseTLSMode_VERIFY_CA
		return nil
	case "insecure":
		*d = DatabaseTLSMode_INSECURE
		return nil
	}
	return trace.BadParameter("DatabaseTLSMode invalid value %v", d)
}

// MarshalJSON supports marshaling enum value into it's string value.
func (s *IAMPolicyStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON supports unmarshaling enum string value back to number.
func (s *IAMPolicyStatus) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	var stringVal string
	if err := json.Unmarshal(data, &stringVal); err != nil {
		return err
	}

	*s = IAMPolicyStatus(IAMPolicyStatus_value[stringVal])
	return nil
}

// IsAuditLogEnabled returns if Oracle Audit Log was enabled
func (o OracleOptions) IsAuditLogEnabled() bool {
	return o.AuditUser != ""
}
