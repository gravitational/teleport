/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package servicecfg

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// DatabasesConfig configures the database proxy service.
type DatabasesConfig struct {
	// Enabled enables the database proxy service.
	Enabled bool
	// Databases is a list of databases proxied by this service.
	Databases []Database
	// ResourceMatchers match cluster database resources.
	ResourceMatchers []services.ResourceMatcher
	// AWSMatchers match AWS hosted databases.
	AWSMatchers []types.AWSMatcher
	// AzureMatchers match Azure hosted databases.
	AzureMatchers []types.AzureMatcher
	// Limiter limits the connection and request rates.
	Limiter limiter.Config
}

// Database represents a single database that's being proxied.
type Database struct {
	// Name is the database name, used to refer to in CLI.
	Name string
	// Description is a free-form database description.
	Description string
	// Protocol is the database type, e.g. postgres or mysql.
	Protocol string
	// URI is the database endpoint to connect to.
	URI string
	// StaticLabels is a map of database static labels.
	StaticLabels map[string]string
	// MySQL are additional MySQL database options.
	MySQL MySQLOptions
	// DynamicLabels is a list of database dynamic labels.
	DynamicLabels services.CommandLabels
	// TLS keeps database connection TLS configuration.
	TLS DatabaseTLS
	// AWS contains AWS specific settings for RDS/Aurora/Redshift databases.
	AWS DatabaseAWS
	// GCP contains GCP specific settings for Cloud SQL databases.
	GCP DatabaseGCP
	// AD contains Active Directory configuration for database.
	AD DatabaseAD
	// Azure contains Azure database configuration.
	Azure DatabaseAzure
	// AdminUser contains information about database admin user.
	AdminUser DatabaseAdminUser
	// Oracle are additional Oracle database options.
	Oracle OracleOptions
}

// DatabaseAdminUser contains information about database admin user.
type DatabaseAdminUser struct {
	// Name is the database admin username (e.g. "postgres").
	Name string
	// DefaultDatabase is the database that the admin user logs into by
	// default.
	//
	// Depending on the database type, this database may be used to store
	// procedures or data for managing database users.
	DefaultDatabase string
}

// OracleOptions are additional Oracle options.
type OracleOptions struct {
	// AuditUser is the Oracle database user privilege to access internal Oracle audit trail.
	AuditUser string
}

// CheckAndSetDefaults validates the database proxy configuration.
func (d *Database) CheckAndSetDefaults() error {
	// Mark the database as coming from the static configuration.
	if d.StaticLabels == nil {
		d.StaticLabels = make(map[string]string)
	}
	d.StaticLabels[types.OriginLabel] = types.OriginConfigFile

	// If AWS account ID is missing, but assume role ARN is given,
	// try to parse the role arn and set the account id to match.
	// TODO(gabrielcorado): move this into the api package.
	if d.AWS.AccountID == "" && d.AWS.AssumeRoleARN != "" {
		parsed, err := awsutils.ParseRoleARN(d.AWS.AssumeRoleARN)
		if err == nil {
			d.AWS.AccountID = parsed.AccountID
		}
	}

	return nil
}

// ToDatabase converts Database to types.Database.
func (d *Database) ToDatabase() (types.Database, error) {
	return types.NewDatabaseV3(types.Metadata{
		Name:        d.Name,
		Description: d.Description,
		Labels:      d.StaticLabels,
	}, types.DatabaseSpecV3{
		Protocol: d.Protocol,
		URI:      d.URI,
		CACert:   string(d.TLS.CACert),
		TLS: types.DatabaseTLS{
			CACert:              string(d.TLS.CACert),
			ServerName:          d.TLS.ServerName,
			Mode:                d.TLS.Mode.ToProto(),
			TrustSystemCertPool: d.TLS.TrustSystemCertPool,
		},
		MySQL: types.MySQLOptions{
			ServerVersion: d.MySQL.ServerVersion,
		},
		AdminUser: &types.DatabaseAdminUser{
			Name:            d.AdminUser.Name,
			DefaultDatabase: d.AdminUser.DefaultDatabase,
		},
		Oracle: convOracleOptions(d.Oracle),
		AWS: types.AWS{
			AccountID:     d.AWS.AccountID,
			AssumeRoleARN: d.AWS.AssumeRoleARN,
			ExternalID:    d.AWS.ExternalID,
			Region:        d.AWS.Region,
			SessionTags:   d.AWS.SessionTags,
			Redshift: types.Redshift{
				ClusterID: d.AWS.Redshift.ClusterID,
			},
			RedshiftServerless: types.RedshiftServerless{
				WorkgroupName: d.AWS.RedshiftServerless.WorkgroupName,
				EndpointName:  d.AWS.RedshiftServerless.EndpointName,
			},
			RDS: types.RDS{
				InstanceID: d.AWS.RDS.InstanceID,
				ClusterID:  d.AWS.RDS.ClusterID,
			},
			ElastiCache: types.ElastiCache{
				ReplicationGroupID: d.AWS.ElastiCache.ReplicationGroupID,
			},
			MemoryDB: types.MemoryDB{
				ClusterName: d.AWS.MemoryDB.ClusterName,
			},
			SecretStore: types.SecretStore{
				KeyPrefix: d.AWS.SecretStore.KeyPrefix,
				KMSKeyID:  d.AWS.SecretStore.KMSKeyID,
			},
		},
		GCP: types.GCPCloudSQL{
			ProjectID:  d.GCP.ProjectID,
			InstanceID: d.GCP.InstanceID,
		},
		DynamicLabels: types.LabelsToV2(d.DynamicLabels),
		AD: types.AD{
			KeytabFile:             d.AD.KeytabFile,
			Krb5File:               d.AD.Krb5File,
			Domain:                 d.AD.Domain,
			SPN:                    d.AD.SPN,
			LDAPCert:               d.AD.LDAPCert,
			KDCHostName:            d.AD.KDCHostName,
			LDAPServiceAccountName: d.AD.LDAPServiceAccountName,
			LDAPServiceAccountSID:  d.AD.LDAPServiceAccountSID,
		},
		Azure: types.Azure{
			ResourceID:    d.Azure.ResourceID,
			IsFlexiServer: d.Azure.IsFlexiServer,
		},
	})
}

func convOracleOptions(o OracleOptions) types.OracleOptions {
	return types.OracleOptions{
		AuditUser: o.AuditUser,
	}
}

// MySQLOptions are additional MySQL options.
type MySQLOptions struct {
	// ServerVersion is the version reported by Teleport DB Proxy on initial handshake.
	ServerVersion string
}

// DatabaseTLS keeps TLS settings used when connecting to database.
type DatabaseTLS struct {
	// Mode is the TLS connection mode. See TLSMode for more details.
	Mode TLSMode
	// ServerName allows providing custom server name.
	// This name will override DNS name when validating certificate presented by the database.
	ServerName string
	// CACert is an optional database CA certificate.
	CACert []byte
	// TrustSystemCertPool allows Teleport to trust certificate authorities
	// available on the host system.
	TrustSystemCertPool bool
}

// DatabaseAWS contains AWS specific settings for RDS/Aurora databases.
type DatabaseAWS struct {
	// Region is the cloud region database is running in when using AWS RDS.
	Region string
	// Redshift contains Redshift specific settings.
	Redshift DatabaseAWSRedshift
	// RDS contains RDS specific settings.
	RDS DatabaseAWSRDS
	// ElastiCache contains ElastiCache specific settings.
	ElastiCache DatabaseAWSElastiCache
	// MemoryDB contains MemoryDB specific settings.
	MemoryDB DatabaseAWSMemoryDB
	// SecretStore contains settings for managing secrets.
	SecretStore DatabaseAWSSecretStore
	// AccountID is the AWS account ID.
	AccountID string
	// AssumeRoleARN is the AWS role to assume to before accessing the database.
	AssumeRoleARN string
	// ExternalID is an optional AWS external ID used to enable assuming an AWS role across accounts.
	ExternalID string
	// RedshiftServerless contains AWS Redshift Serverless specific settings.
	RedshiftServerless DatabaseAWSRedshiftServerless
	// SessionTags is a list of AWS STS session tags.
	SessionTags map[string]string
}

// DatabaseAWSRedshift contains AWS Redshift specific settings.
type DatabaseAWSRedshift struct {
	// ClusterID is the Redshift cluster identifier.
	ClusterID string
}

// DatabaseAWSRedshiftServerless contains AWS Redshift Serverless specific settings.
type DatabaseAWSRedshiftServerless struct {
	// WorkgroupName is the Redshift Serverless workgroup name.
	WorkgroupName string
	// EndpointName is the Redshift Serverless VPC endpoint name.
	EndpointName string
}

// DatabaseAWSRDS contains AWS RDS specific settings.
type DatabaseAWSRDS struct {
	// InstanceID is the RDS instance identifier.
	InstanceID string
	// ClusterID is the RDS cluster (Aurora) identifier.
	ClusterID string
}

// DatabaseAWSElastiCache contains settings for ElastiCache databases.
type DatabaseAWSElastiCache struct {
	// ReplicationGroupID is the ElastiCache replication group ID.
	ReplicationGroupID string
}

// DatabaseAWSMemoryDB contains settings for MemoryDB databases.
type DatabaseAWSMemoryDB struct {
	// ClusterName is the MemoryDB cluster name.
	ClusterName string
}

// DatabaseAWSSecretStore contains secret store configurations.
type DatabaseAWSSecretStore struct {
	// KeyPrefix specifies the secret key prefix.
	KeyPrefix string
	// KMSKeyID specifies the AWS KMS key for encryption.
	KMSKeyID string
}

// DatabaseGCP contains GCP specific settings for Cloud SQL databases.
type DatabaseGCP struct {
	// ProjectID is the GCP project ID where the database is deployed.
	ProjectID string
	// InstanceID is the Cloud SQL instance ID.
	InstanceID string
}

// DatabaseAD contains database Active Directory configuration.
type DatabaseAD struct {
	// KeytabFile is the path to the Kerberos keytab file.
	KeytabFile string
	// Krb5File is the path to the Kerberos configuration file. Defaults to /etc/krb5.conf.
	Krb5File string
	// Domain is the Active Directory domain the database resides in.
	Domain string
	// SPN is the service principal name for the database.
	SPN string
	// LDAPCert is the Active Directory LDAP Certificate.
	LDAPCert string
	// KDCHostName is the Key Distribution Center Hostname for x509 authentication
	KDCHostName string
	// LDAPServiceAccountName is the name of service account for performing LDAP queries. Required for x509 Auth / PKINIT.
	LDAPServiceAccountName string
	// LDAPServiceAccountSID is the SID of service account for performing LDAP queries. Required for x509 Auth / PKINIT.
	LDAPServiceAccountSID string
}

// DatabaseAzure contains Azure database configuration.
type DatabaseAzure struct {
	// ResourceID is the Azure fully qualified ID for the resource.
	ResourceID string
	// IsFlexiServer is true if the database is an Azure Flexible server.
	IsFlexiServer bool
}
