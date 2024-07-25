/**
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

import { DbProtocol } from 'shared/services/databases';

import { Platform } from 'design/platform';

import { DiscoverEventResource } from 'teleport/services/userEvent';

import { ResourceKind } from '../Shared/ResourceKind';

import { ResourceSpec, DatabaseLocation, DatabaseEngine } from './types';

const baseDatabaseKeywords = 'db database databases';
const awsKeywords = baseDatabaseKeywords + 'aws amazon web services';
const gcpKeywords = baseDatabaseKeywords + 'gcp google cloud provider';
const selfhostedKeywords = baseDatabaseKeywords + 'self hosted self-hosted';
const azureKeywords = baseDatabaseKeywords + 'microsoft azure';

function getDbAccessDocLink(guide: string) {
  return `https://goteleport.com/docs/database-access/guides/${guide}`;
}

// DATABASES_UNGUIDED_DOC are documentations that is not specific
// to one type of database.
export const DATABASES_UNGUIDED_DOC: ResourceSpec[] = [
  {
    dbMeta: { location: DatabaseLocation.Aws, engine: DatabaseEngine.Doc },
    name: 'RDS Proxy PostgreSQL',
    keywords: awsKeywords + 'rds proxy postgresql',
    kind: ResourceKind.Database,
    icon: 'aws',
    unguidedLink: getDbAccessDocLink('rds-proxy-postgres'),
    // TODO(lisa): add a new usage event
    event: DiscoverEventResource.DatabaseDocRdsProxy,
  },
  {
    dbMeta: { location: DatabaseLocation.Aws, engine: DatabaseEngine.Doc },
    name: 'RDS Proxy SQL Server',
    keywords: awsKeywords + 'rds proxy sql server sqlserver',
    kind: ResourceKind.Database,
    icon: 'aws',
    unguidedLink: getDbAccessDocLink('rds-proxy-sqlserver'),
    // TODO(lisa): add a new usage event
    event: DiscoverEventResource.DatabaseDocRdsProxy,
  },
  {
    dbMeta: { location: DatabaseLocation.Aws, engine: DatabaseEngine.Doc },
    name: 'RDS Proxy MariaDB/MySQL',
    keywords: awsKeywords + 'rds proxy mariadb mysql',
    kind: ResourceKind.Database,
    icon: 'aws',
    unguidedLink: getDbAccessDocLink('rds-proxy-mysql'),
    // TODO(lisa): add a new usage event
    event: DiscoverEventResource.DatabaseDocRdsProxy,
  },
  {
    dbMeta: { location: DatabaseLocation.TODO, engine: DatabaseEngine.Doc },
    name: 'High Availability',
    keywords: baseDatabaseKeywords + 'high availability ha',
    kind: ResourceKind.Database,
    icon: 'database',
    unguidedLink: getDbAccessDocLink('ha'),
    event: DiscoverEventResource.DatabaseDocHighAvailability,
  },
  {
    dbMeta: { location: DatabaseLocation.TODO, engine: DatabaseEngine.Doc },
    name: 'Dynamic Registration',
    keywords: baseDatabaseKeywords + 'dynamic registration',
    kind: ResourceKind.Database,
    icon: 'database',
    unguidedLink: getDbAccessDocLink('dynamic-registration'),
    event: DiscoverEventResource.DatabaseDocDynamicRegistration,
  },
];

export const DATABASES_UNGUIDED: ResourceSpec[] = [
  {
    dbMeta: { location: DatabaseLocation.Aws, engine: DatabaseEngine.DynamoDb },
    name: 'DynamoDB',
    keywords: awsKeywords + 'dynamodb',
    kind: ResourceKind.Database,
    icon: 'dynamo',
    unguidedLink: getDbAccessDocLink('aws-dynamodb'),
    event: DiscoverEventResource.DatabaseDynamoDb,
  },
  {
    dbMeta: { location: DatabaseLocation.Aws, engine: DatabaseEngine.Redis },
    name: 'ElastiCache & MemoryDB',
    keywords: awsKeywords + 'elasticache memorydb redis',
    kind: ResourceKind.Database,
    icon: 'aws',
    unguidedLink: getDbAccessDocLink('redis-aws'),
    event: DiscoverEventResource.DatabaseRedisElasticache,
  },
  {
    dbMeta: {
      location: DatabaseLocation.Aws,
      engine: DatabaseEngine.Cassandra,
    },
    name: 'Keyspaces (Apache Cassandra)',
    keywords: awsKeywords + 'keyspaces apache cassandra',
    kind: ResourceKind.Database,
    icon: 'aws',
    unguidedLink: getDbAccessDocLink('aws-cassandra-keyspaces'),
    event: DiscoverEventResource.DatabaseCassandraKeyspaces,
  },
  {
    dbMeta: { location: DatabaseLocation.Aws, engine: DatabaseEngine.Redshift },
    name: 'Redshift PostgreSQL',
    keywords: awsKeywords + 'redshift postgresql',
    kind: ResourceKind.Database,
    icon: 'redshift',
    unguidedLink: getDbAccessDocLink('postgres-redshift'),
    event: DiscoverEventResource.DatabasePostgresRedshift,
  },
  {
    dbMeta: { location: DatabaseLocation.Aws, engine: DatabaseEngine.Redshift },
    name: 'Redshift Serverless',
    keywords: awsKeywords + 'redshift serverless postgresql',
    kind: ResourceKind.Database,
    icon: 'redshift',
    unguidedLink: getDbAccessDocLink('redshift-serverless'),
    event: DiscoverEventResource.DatabasePostgresRedshiftServerless,
  },
  {
    dbMeta: { location: DatabaseLocation.Azure, engine: DatabaseEngine.Redis },
    name: 'Cache for Redis',
    keywords: azureKeywords + 'cache redis',
    kind: ResourceKind.Database,
    icon: 'azure',
    unguidedLink: getDbAccessDocLink('azure-redis'),
    event: DiscoverEventResource.DatabaseRedisAzureCache,
  },
  {
    dbMeta: {
      location: DatabaseLocation.Azure,
      engine: DatabaseEngine.Postgres,
    },
    name: 'PostgreSQL',
    keywords: azureKeywords + 'postgresql',
    kind: ResourceKind.Database,
    icon: 'azure',
    unguidedLink: getDbAccessDocLink('azure-postgres-mysql'),
    event: DiscoverEventResource.DatabasePostgresAzure,
  },
  {
    dbMeta: { location: DatabaseLocation.Azure, engine: DatabaseEngine.MySql },
    name: 'MySQL',
    keywords: azureKeywords + 'mysql',
    kind: ResourceKind.Database,
    icon: 'azure',
    unguidedLink: getDbAccessDocLink('azure-postgres-mysql'),
    event: DiscoverEventResource.DatabaseMysqlAzure,
  },
  {
    dbMeta: {
      location: DatabaseLocation.Azure,
      engine: DatabaseEngine.SqlServer,
    },
    name: 'SQL Server',
    keywords:
      azureKeywords + 'active directory ad sql server sqlserver preview',
    kind: ResourceKind.Database,
    icon: 'azure',
    unguidedLink: getDbAccessDocLink('azure-sql-server-ad'),
    event: DiscoverEventResource.DatabaseSqlServerAzure,
    platform: Platform.Windows,
  },
  {
    dbMeta: {
      location: DatabaseLocation.Microsoft,
      engine: DatabaseEngine.SqlServer,
    },
    name: 'SQL Server',
    keywords:
      baseDatabaseKeywords +
      'microsoft active directory ad sql server sqlserver preview',
    kind: ResourceKind.Database,
    icon: 'windows',
    unguidedLink: getDbAccessDocLink('sql-server-ad'),
    event: DiscoverEventResource.DatabaseSqlServerMicrosoft,
    platform: Platform.Windows,
  },
  {
    dbMeta: { location: DatabaseLocation.Gcp, engine: DatabaseEngine.MySql },
    name: 'Cloud SQL MySQL',
    keywords: gcpKeywords + 'mysql',
    kind: ResourceKind.Database,
    icon: 'googlecloud',
    unguidedLink: getDbAccessDocLink('mysql-cloudsql'),
    event: DiscoverEventResource.DatabaseMysqlGcp,
  },
  {
    dbMeta: { location: DatabaseLocation.Gcp, engine: DatabaseEngine.Postgres },
    name: 'Cloud SQL PostgreSQL',
    keywords: gcpKeywords + 'postgresql',
    kind: ResourceKind.Database,
    icon: 'googlecloud',
    unguidedLink: getDbAccessDocLink('postgres-cloudsql'),
    event: DiscoverEventResource.DatabasePostgresGcp,
  },
  {
    dbMeta: {
      location: DatabaseLocation.TODO,
      engine: DatabaseEngine.MongoDb,
    },
    name: 'MongoDB Atlas',
    keywords: baseDatabaseKeywords + 'mongodb atlas',
    kind: ResourceKind.Database,
    icon: 'mongo',
    unguidedLink: getDbAccessDocLink('mongodb-atlas'),
    event: DiscoverEventResource.DatabaseMongodbAtlas,
  },
  {
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.Cassandra,
    },
    name: 'Cassandra & ScyllaDB',
    keywords: selfhostedKeywords + 'cassandra scylladb',
    kind: ResourceKind.Database,
    icon: 'selfhosted',
    unguidedLink: getDbAccessDocLink('cassandra-self-hosted'),
    event: DiscoverEventResource.DatabaseCassandraSelfHosted,
  },
  {
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.CoackroachDb,
    },
    name: 'CockroachDB',
    keywords: selfhostedKeywords + 'cockroachdb',
    kind: ResourceKind.Database,
    icon: 'cockroach',
    unguidedLink: getDbAccessDocLink('cockroachdb-self-hosted'),
    event: DiscoverEventResource.DatabaseCockroachDbSelfHosted,
  },
  {
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.ElasticSearch,
    },
    name: 'Elasticsearch',
    keywords: selfhostedKeywords + 'elasticsearch',
    kind: ResourceKind.Database,
    icon: 'selfhosted',
    unguidedLink: getDbAccessDocLink('elastic'),
    event: DiscoverEventResource.DatabaseElasticSearchSelfHosted,
  },
  {
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.MongoDb,
    },
    name: 'MongoDB',
    keywords: selfhostedKeywords + 'mongodb',
    kind: ResourceKind.Database,
    icon: 'mongo',
    unguidedLink: getDbAccessDocLink('mongodb-self-hosted'),
    event: DiscoverEventResource.DatabaseMongodbSelfHosted,
  },
  {
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.Redis,
    },
    name: 'Redis',
    keywords: selfhostedKeywords + 'redis',
    kind: ResourceKind.Database,
    icon: 'selfhosted',
    unguidedLink: getDbAccessDocLink('redis'),
    event: DiscoverEventResource.DatabaseRedisSelfHosted,
  },
  {
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.Redis,
    },
    name: 'Redis Cluster',
    keywords: selfhostedKeywords + 'redis cluster',
    kind: ResourceKind.Database,
    icon: 'selfhosted',
    unguidedLink: getDbAccessDocLink('redis-cluster'),
    event: DiscoverEventResource.DatabaseRedisClusterSelfHosted,
  },
  {
    dbMeta: {
      location: DatabaseLocation.TODO,
      engine: DatabaseEngine.Snowflake,
    },
    name: 'Snowflake',
    keywords: baseDatabaseKeywords + 'snowflake preview',
    kind: ResourceKind.Database,
    icon: 'snowflake',
    unguidedLink: getDbAccessDocLink('snowflake'),
    event: DiscoverEventResource.DatabaseSnowflake,
  },
];

export const DATABASES: ResourceSpec[] = [
  {
    dbMeta: {
      location: DatabaseLocation.Aws,
      engine: DatabaseEngine.Postgres,
    },
    name: 'RDS PostgreSQL',
    keywords: awsKeywords + 'rds postgresql',
    kind: ResourceKind.Database,
    icon: 'aws',
    event: DiscoverEventResource.DatabasePostgresRds,
  },
  {
    dbMeta: {
      location: DatabaseLocation.Aws,
      engine: DatabaseEngine.AuroraPostgres,
    },
    name: 'Aurora PostgreSQL',
    keywords: awsKeywords + 'aurora postgresql',
    kind: ResourceKind.Database,
    icon: 'aws',
    event: DiscoverEventResource.DatabasePostgresRds,
  },
  {
    dbMeta: { location: DatabaseLocation.Aws, engine: DatabaseEngine.MySql },
    name: 'RDS MySQL/MariaDB',
    keywords: awsKeywords + 'rds mysql mariadb',
    kind: ResourceKind.Database,
    icon: 'aws',
    event: DiscoverEventResource.DatabaseMysqlRds,
  },
  {
    dbMeta: {
      location: DatabaseLocation.Aws,
      engine: DatabaseEngine.AuroraMysql,
    },
    name: 'Aurora MySQL/MariaDB',
    keywords: awsKeywords + 'aurora mysql mariadb',
    kind: ResourceKind.Database,
    icon: 'aws',
    event: DiscoverEventResource.DatabaseMysqlRds,
  },
  {
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.Postgres,
    },
    name: 'PostgreSQL',
    keywords: selfhostedKeywords + 'postgresql',
    kind: ResourceKind.Database,
    icon: 'postgres',
    event: DiscoverEventResource.DatabasePostgresSelfHosted,
  },
  {
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.MySql,
    },
    name: 'MySQL/MariaDB',
    keywords: selfhostedKeywords + 'mysql mariadb',
    kind: ResourceKind.Database,
    icon: 'selfhosted',
    event: DiscoverEventResource.DatabaseMysqlSelfHosted,
  },
];

export function getDatabaseProtocol(engine: DatabaseEngine): DbProtocol {
  switch (engine) {
    case DatabaseEngine.Postgres:
      return 'postgres';
    case DatabaseEngine.MySql:
      return 'mysql';
  }

  return '' as any;
}

export function getDefaultDatabasePort(engine: DatabaseEngine): string {
  switch (engine) {
    case DatabaseEngine.Postgres:
      return '5432';
    case DatabaseEngine.MySql:
      return '3306';
  }
  return '';
}
