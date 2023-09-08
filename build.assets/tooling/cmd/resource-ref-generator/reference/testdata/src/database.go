// Teleport
// Copyright (C) 2023  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package typestest

import (
	fmt "fmt"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

var KindDatabase string = "database"
var V3 string = "v3"

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

	// Admin user (for automatic user provisioning) is only supported for
	// PostgreSQL currently.
	if d.GetAdminUser() != "" && !d.SupportsAutoUsers() {
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
