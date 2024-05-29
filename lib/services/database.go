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

package services

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"net/url"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"

	"github.com/gravitational/teleport/api/types"
	azureutils "github.com/gravitational/teleport/api/utils/azure"
	gcputils "github.com/gravitational/teleport/api/utils/gcp"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common/enterprise"
	"github.com/gravitational/teleport/lib/srv/db/redis/connection"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// DatabaseGetter defines interface for fetching database resources.
type DatabaseGetter interface {
	// GetDatabases returns all database resources.
	GetDatabases(context.Context) ([]types.Database, error)
	// GetDatabase returns the specified database resource.
	GetDatabase(ctx context.Context, name string) (types.Database, error)
}

// Databases defines an interface for managing database resources.
type Databases interface {
	// DatabaseGetter provides methods for fetching database resources.
	DatabaseGetter
	// CreateDatabase creates a new database resource.
	CreateDatabase(context.Context, types.Database) error
	// UpdateDatabase updates an existing database resource.
	UpdateDatabase(context.Context, types.Database) error
	// DeleteDatabase removes the specified database resource.
	DeleteDatabase(ctx context.Context, name string) error
	// DeleteAllDatabases removes all database resources.
	DeleteAllDatabases(context.Context) error
}

// MarshalDatabase marshals the database resource to JSON.
func MarshalDatabase(database types.Database, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch database := database.(type) {
	case *types.DatabaseV3:
		if err := database.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoResourceID(cfg.PreserveResourceID, database))
	default:
		return nil, trace.BadParameter("unsupported database resource %T", database)
	}
}

// UnmarshalDatabase unmarshals the database resource from JSON.
func UnmarshalDatabase(data []byte, opts ...MarshalOption) (types.Database, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing database resource data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V3:
		var database types.DatabaseV3
		if err := utils.FastUnmarshal(data, &database); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := database.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			database.SetResourceID(cfg.ID)
		}
		if cfg.Revision != "" {
			database.SetRevision(cfg.Revision)
		}
		if !cfg.Expires.IsZero() {
			database.SetExpiry(cfg.Expires)
		}
		return &database, nil
	}
	return nil, trace.BadParameter("unsupported database resource version %q", h.Version)
}

// ValidateDatabase validates a types.Database.
func ValidateDatabase(db types.Database) error {
	if err := enterprise.ProtocolValidation(db.GetProtocol()); err != nil {
		return trace.Wrap(err)
	}

	if err := CheckAndSetDefaults(db); err != nil {
		return trace.Wrap(err)
	}

	if !slices.Contains(defaults.DatabaseProtocols, db.GetProtocol()) {
		return trace.BadParameter("unsupported database %q protocol %q, supported are: %v", db.GetName(), db.GetProtocol(), defaults.DatabaseProtocols)
	}

	// For MongoDB we support specifying either server address or connection
	// string in the URI which is useful when connecting to a replica set.
	if db.GetProtocol() == defaults.ProtocolMongoDB &&
		(strings.HasPrefix(db.GetURI(), connstring.SchemeMongoDB+"://") ||
			strings.HasPrefix(db.GetURI(), connstring.SchemeMongoDBSRV+"://")) {
		if err := validateMongoDB(db); err != nil {
			return trace.Wrap(err)
		}
	} else if db.GetProtocol() == defaults.ProtocolRedis {
		_, err := connection.ParseRedisAddress(db.GetURI())
		if err != nil {
			return trace.BadParameter("invalid Redis database %q address: %q, error: %v", db.GetName(), db.GetURI(), err)
		}
	} else if db.GetProtocol() == defaults.ProtocolSnowflake {
		if !strings.Contains(db.GetURI(), defaults.SnowflakeURL) {
			return trace.BadParameter("Snowflake address should contain " + defaults.SnowflakeURL)
		}
	} else if db.GetProtocol() == defaults.ProtocolClickHouse || db.GetProtocol() == defaults.ProtocolClickHouseHTTP {
		if err := validateClickhouseURI(db); err != nil {
			return trace.Wrap(err)
		}
	} else if db.GetProtocol() == defaults.ProtocolSQLServer {
		if err := ValidateSQLServerURI(db.GetURI()); err != nil {
			return trace.BadParameter("invalid SQL Server address: %v", err)
		}
	} else if db.GetProtocol() == defaults.ProtocolSpanner {
		if !gcputils.IsSpannerEndpoint(db.GetURI()) {
			return trace.BadParameter("GCP Spanner database %q address should be %q",
				db.GetName(), gcputils.SpannerEndpoint)
		}
	} else if needsURIValidation(db) {
		if _, _, err := net.SplitHostPort(db.GetURI()); err != nil {
			return trace.BadParameter("invalid database %q address %q: %v", db.GetName(), db.GetURI(), err)
		}
	}

	if db.GetTLS().CACert != "" {
		if _, err := tlsca.ParseCertificatePEM([]byte(db.GetTLS().CACert)); err != nil {
			return trace.BadParameter("provided database %q CA doesn't appear to be a valid x509 certificate: %v", db.GetName(), err)
		}
	}

	// Validate Active Directory specific configuration, when Kerberos auth is required.
	if needsADValidation(db) {
		if db.GetAD().KeytabFile == "" && db.GetAD().KDCHostName == "" {
			return trace.BadParameter("either keytab file path or kdc_host_name must be provided for database %q, both are missing", db.GetName())
		}
		if db.GetAD().Krb5File == "" {
			return trace.BadParameter("missing Kerberos config file path for database %q", db.GetName())
		}
		if db.GetAD().Domain == "" {
			return trace.BadParameter("missing Active Directory domain for database %q", db.GetName())
		}
		if db.GetAD().SPN == "" {
			return trace.BadParameter("missing service principal name for database %q", db.GetName())
		}
		if db.GetAD().KDCHostName != "" {
			if db.GetAD().LDAPCert == "" {
				return trace.BadParameter("missing LDAP certificate for x509 authentication for database %q", db.GetName())
			}
		}
	}

	awsMeta := db.GetAWS()
	if awsMeta.AssumeRoleARN != "" {
		if awsMeta.AccountID == "" {
			return trace.BadParameter("database %q missing AWS account ID", db.GetName())
		}
		parsed, err := awsutils.ParseRoleARN(awsMeta.AssumeRoleARN)
		if err != nil {
			return trace.BadParameter("database %q assume_role_arn %q is invalid: %v",
				db.GetName(), awsMeta.AssumeRoleARN, err)
		}
		err = awsutils.CheckARNPartitionAndAccount(parsed, awsMeta.Partition(), awsMeta.AccountID)
		if err != nil {
			return trace.BadParameter("database %q is incompatible with AWS assume_role_arn %q: %v",
				db.GetName(), awsMeta.AssumeRoleARN, err)
		}
	}

	return nil
}

// needsADValidation returns whether a database AD configuration needs to
// be validated.
func needsADValidation(db types.Database) bool {
	if db.GetProtocol() != defaults.ProtocolSQLServer {
		return false
	}

	// Domain is always required when configuring the AD section, so we assume
	// users intend to use Kerberos authentication if the configuration has it.
	if db.GetAD().Domain != "" {
		return true
	}

	// Azure-hosted databases and RDS Proxy support other authentication
	// methods, and do not require this section to be validated.
	if strings.Contains(db.GetURI(), azureutils.MSSQLEndpointSuffix) || db.GetAWS().RDSProxy.Name != "" {
		return false
	}

	return true
}

func validateClickhouseURI(db types.Database) error {
	u, err := url.Parse(db.GetURI())
	if err != nil {
		return trace.BadParameter("failed to parse uri: %v", err)
	}
	var requiredSchema string
	if db.GetProtocol() == defaults.ProtocolClickHouse {
		requiredSchema = "clickhouse"
	}
	if db.GetProtocol() == defaults.ProtocolClickHouseHTTP {
		requiredSchema = "https"
	}
	if u.Scheme != requiredSchema {
		return trace.BadParameter("invalid uri schema: %s for %v database protocol", u.Scheme, db.GetProtocol())
	}
	return nil
}

// needsURIValidation returns whether a database URI needs to be validated.
func needsURIValidation(db types.Database) bool {
	switch db.GetProtocol() {
	case defaults.ProtocolCassandra, defaults.ProtocolDynamoDB:
		// cloud hosted Cassandra doesn't require URI validation.
		return db.GetAWS().Region == "" || db.GetAWS().AccountID == ""
	default:
		return true
	}
}

// validateMongoDB validates MongoDB URIs with "mongodb" schemes.
func validateMongoDB(db types.Database) error {
	connString, err := connstring.ParseAndValidate(db.GetURI())
	// connstring.ParseAndValidate requires DNS resolution on TXT/SRV records
	// for a full validation for "mongodb+srv" URIs. We will try to skip the
	// DNS errors here by replacing the scheme and then ParseAndValidate again
	// to validate as much as we can.
	if isDNSError(err) {
		log.Warnf("MongoDB database %q (connection string %q) failed validation with DNS error: %v.", db.GetName(), db.GetURI(), err)

		connString, err = connstring.ParseAndValidate(strings.Replace(
			db.GetURI(),
			connstring.SchemeMongoDBSRV+"://",
			connstring.SchemeMongoDB+"://",
			1,
		))
	}
	if err != nil {
		return trace.BadParameter("invalid MongoDB database %q connection string %q: %v", db.GetName(), db.GetURI(), err)
	}

	// Validate read preference to catch typos early.
	if connString.ReadPreference != "" {
		if _, err := readpref.ModeFromString(connString.ReadPreference); err != nil {
			return trace.BadParameter("invalid MongoDB database %q read preference %q", db.GetName(), connString.ReadPreference)
		}
	}
	return nil
}

// ValidateSQLServerURI validates SQL Server URI and returns host and
// port.
//
// Since Teleport only supports SQL Server authentcation using AD (self-hosted
// or Azure) the database URI must include: computer name, domain and port.
//
// A few examples of valid URIs:
// - computer.ad.example.com:1433
// - computer.domain.com:1433
func ValidateSQLServerURI(uri string) error {
	// sqlServerSchema is the SQL Server schema.
	const sqlServerSchema = "mssql"

	// Add a temporary schema to make a valid URL for url.Parse if schema is
	// not found.
	if !strings.Contains(uri, "://") {
		uri = sqlServerSchema + "://" + uri
	}

	parsedURI, err := url.Parse(uri)
	if err != nil {
		return trace.BadParameter("unable to parse database address: %s", err)
	}

	if parsedURI.Scheme != sqlServerSchema {
		return trace.BadParameter("only %q is supported as database address schema", sqlServerSchema)
	}

	if parsedURI.Port() == "" {
		return trace.BadParameter("database address must include port")
	}

	if parsedURI.Path != "" {
		return trace.BadParameter("database address with database name is not supported")
	}

	if _, err := netip.ParseAddr(parsedURI.Hostname()); err == nil {
		return trace.BadParameter("database address as IP is not supported, use URI with domain and computer name instead")
	}

	parts := strings.Split(parsedURI.Hostname(), ".")
	if len(parts) < 3 {
		return trace.BadParameter("database address must include domain and computer name")
	}

	return nil
}

func isDNSError(err error) bool {
	if err == nil {
		return false
	}

	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr)
}

const (
	// RDSDescribeTypeInstance is used to filter for Instances type of RDS DBs when describing RDS Databases.
	RDSDescribeTypeInstance = "instance"
	// RDSDescribeTypeCluster is used to filter for Clusters type of RDS DBs when describing RDS Databases.
	RDSDescribeTypeCluster = "cluster"
)

const (
	// RDSEngineMySQL is RDS engine name for MySQL instances.
	RDSEngineMySQL = "mysql"
	// RDSEnginePostgres is RDS engine name for Postgres instances.
	RDSEnginePostgres = "postgres"
	// RDSEngineMariaDB is RDS engine name for MariaDB instances.
	RDSEngineMariaDB = "mariadb"
	// RDSEngineAurora is RDS engine name for Aurora MySQL 5.6 compatible clusters.
	// This reached EOF on Feb 28, 2023.
	// https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/Aurora.MySQL56.EOL.html
	RDSEngineAurora = "aurora"
	// RDSEngineAuroraMySQL is RDS engine name for Aurora MySQL 5.7 compatible clusters.
	RDSEngineAuroraMySQL = "aurora-mysql"
	// RDSEngineAuroraPostgres is RDS engine name for Aurora Postgres clusters.
	RDSEngineAuroraPostgres = "aurora-postgresql"
)

const (
	// RDSEngineModeProvisioned is the RDS engine mode for provisioned Aurora clusters
	RDSEngineModeProvisioned = "provisioned"
	// RDSEngineModeServerless is the RDS engine mode for Aurora Serverless DB clusters
	RDSEngineModeServerless = "serverless"
	// RDSEngineModeParallelQuery is the RDS engine mode for Aurora MySQL clusters with parallel query enabled
	RDSEngineModeParallelQuery = "parallelquery"
)

const (
	// RDSProxyMySQLPort is the port that RDS Proxy listens on for MySQL connections.
	RDSProxyMySQLPort = 3306
	// RDSProxyPostgresPort is the port that RDS Proxy listens on for Postgres connections.
	RDSProxyPostgresPort = 5432
	// RDSProxySQLServerPort is the port that RDS Proxy listens on for SQL Server connections.
	RDSProxySQLServerPort = 1433
)

const (
	// AzureEngineMySQL is the Azure engine name for MySQL single-server instances.
	AzureEngineMySQL = "Microsoft.DBforMySQL/servers"
	// AzureEngineMySQLFlex is the Azure engine name for MySQL flexible-server instances.
	AzureEngineMySQLFlex = "Microsoft.DBforMySQL/flexibleServers"
	// AzureEnginePostgres is the Azure engine name for PostgreSQL single-server instances.
	AzureEnginePostgres = "Microsoft.DBforPostgreSQL/servers"
	// AzureEnginePostgresFlex is the Azure engine name for PostgreSQL flexible-server instances.
	AzureEnginePostgresFlex = "Microsoft.DBforPostgreSQL/flexibleServers"
)

const (
	// RedshiftServerlessWorkgroupEndpoint is the endpoint type for workgroups.
	RedshiftServerlessWorkgroupEndpoint = "workgroup"
	// RedshiftServerlessVPCEndpoint is the endpoint type for VCP endpoints.
	RedshiftServerlessVPCEndpoint = "vpc-endpoint"
)
