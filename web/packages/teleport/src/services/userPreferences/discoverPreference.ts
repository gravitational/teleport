/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

/**
 * Used to uniquely identify resource guides. These UID's will
 * be stored in the backend as user preference to preserve
 * which resource guides user wanted to "pin".
 *
 * There is no specific format to use, just ensure that enum values
 * are unique.
 *
 * Existing enum values must not be modified.
 */
export enum DiscoverGuideId {
  // Servers:
  ServerLinuxUbuntu = 'server-linux-ubuntu',
  ServerLinuxDebian = 'server-linux-debian',
  ServerLinuxRhelCentos = 'server-linux-rhel-centos',
  ServerLinuxAmazon = 'server-linux-amazon',
  ServerMac = 'server-mac',
  ServerAwsEc2Ssm = 'server-aws-ec2-ssm',
  ConnectMyComputer = 'connect-my-computer',

  // Applications:
  ApplicationWebHttpProxy = 'application-web-http-proxy',
  ApplicationAwsCliConsole = 'application-aws-cli-console',
  ApplicationSamlGeneric = 'application-saml-generic',
  ApplicationSamlGrafana = 'application-saml-grafana',
  ApplicationSamlWorkforceIdentityFederation = 'application-saml-workforce-identity-federation',

  // Windows Desktops:
  WindowsDesktopsActiveDirectory = 'windows-desktops-active-directory',
  WindowsDesktopsLocal = 'windows-desktops-local',

  // Kubernetes:
  Kubernetes = 'kubernetes',
  KubernetesAwsEks = 'kubernetes-aws-eks',

  // Databases:
  DatabaseAwsDynamoDb = 'database-aws-dynamo-db',
  DatabaseAwsElastiCacheMemoryDb = 'database-aws-elasticache-memorydb',
  DatabaseAwsCassandraKeyspaces = 'database-aws-cassandra-keyspaces',
  DatabaseAwsRedshiftServerless = 'database-aws-redshift-serverless',
  DatabaseAwsSqlServerAd = 'database-aws-sql-server-ad',
  DatabaseAwsPostgresRedshift = 'database-aws-postgres-redshift',
  DatabaseAwsRdsPostgres = 'database-aws-rds-postgres',
  DatabaseAwsRdsProxyPostgres = 'database-aws-rds-proxy-postgres',
  DatabaseAwsRdsAuroraPostgres = 'database-aws-rds-aurora-postgres',
  DatabaseAwsRdsProxySqlServer = 'database-aws-rds-proxy-sql-server',
  DatabaseAwsRdsProxyMariaMySql = 'database-aws-rds-proxy-maria-mysql',
  DatabaseAwsRdsAuroraMysql = 'database-aws-rds-aurora-mysql',
  DatabaseAwsRdsMysqlMariaDb = 'database-aws-rds-mysql-mariadb',

  DatabaseHighAvailability = 'database-high-availability',
  DatabaseDynamicRegistration = 'database-dynamic-registration',

  DatabaseAzureRedis = 'database-azure-redis',
  DatabaseAzurePostgres = 'database-azure-postgres',
  DatabaseAzureMysql = 'database-azure-mysql',
  DatabaseAzureSqlServerAd = 'database-azure-sql-server-ad',

  DatabaseGcpMysqlCloudSql = 'database-gcp-mysql-cloud-sql',
  DatabaseGcpPostgresCloudSql = 'database-gcp-postgres-cloud-sql',

  DatabaseMongoAtlas = 'database-mongo-atlas',
  DatabaseCassandraScyllaDb = 'database-cassandra-scylladb',
  DatabaseCockroachDb = 'database-cockroachdb',
  DatabaseElasticSearch = 'database-elasticsearch',
  DatabaseMongoDb = 'database-mongodb',
  DatabaseRedis = 'database-redis',
  DatabaseRedisCluster = 'database-redis-cluster',
  DatabaseSnowflake = 'database-snowflake',
  DatabasePostgres = 'database-postgres',
  DatabaseMysql = 'database-mysql',
}
