/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { DbProtocol } from 'shared/services/databases';

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
    name: 'RDS Proxy',
    keywords: awsKeywords + 'rds proxy',
    kind: ResourceKind.Database,
    icon: 'Aws',
    unguidedLink: getDbAccessDocLink('rds-proxy'),
    event: DiscoverEventResource.DatabaseDocRdsProxy,
  },
  {
    dbMeta: { location: DatabaseLocation.TODO, engine: DatabaseEngine.Doc },
    name: 'High Availability',
    keywords: baseDatabaseKeywords + 'high availability ha',
    kind: ResourceKind.Database,
    icon: 'Database',
    unguidedLink: getDbAccessDocLink('ha'),
    event: DiscoverEventResource.DatabaseDocHighAvailability,
  },
  {
    dbMeta: { location: DatabaseLocation.TODO, engine: DatabaseEngine.Doc },
    name: 'Dynamic Registration',
    keywords: baseDatabaseKeywords + 'dynamic registration',
    kind: ResourceKind.Database,
    icon: 'Database',
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
    icon: 'Dynamo',
    unguidedLink: getDbAccessDocLink('aws-dynamodb'),
    event: DiscoverEventResource.DatabaseDynamoDb,
  },
  {
    dbMeta: { location: DatabaseLocation.Aws, engine: DatabaseEngine.Redis },
    name: 'ElastiCache & MemoryDB',
    keywords: awsKeywords + 'elasticache memorydb redis',
    kind: ResourceKind.Database,
    icon: 'Aws',
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
    icon: 'Aws',
    unguidedLink: getDbAccessDocLink('aws-cassandra-keyspaces'),
    event: DiscoverEventResource.DatabaseCassandraKeyspaces,
  },
  {
    dbMeta: { location: DatabaseLocation.Aws, engine: DatabaseEngine.Redshift },
    name: 'Redshift PostgreSQL',
    keywords: awsKeywords + 'redshift postgresql',
    kind: ResourceKind.Database,
    icon: 'Redshift',
    unguidedLink: getDbAccessDocLink('postgres-redshift'),
    event: DiscoverEventResource.DatabasePostgresRedshift,
  },
  {
    dbMeta: { location: DatabaseLocation.Aws, engine: DatabaseEngine.Redshift },
    name: 'Redshift Serverless',
    keywords: awsKeywords + 'redshift serverless postgresql',
    kind: ResourceKind.Database,
    icon: 'Redshift',
    unguidedLink: getDbAccessDocLink('redshift-serverless'),
    event: DiscoverEventResource.DatabasePostgresRedshiftServerless,
  },
  {
    dbMeta: { location: DatabaseLocation.Azure, engine: DatabaseEngine.Redis },
    name: 'Cache for Redis',
    keywords: azureKeywords + 'cache redis',
    kind: ResourceKind.Database,
    icon: 'Azure',
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
    icon: 'Azure',
    unguidedLink: getDbAccessDocLink('azure-postgres-mysql'),
    event: DiscoverEventResource.DatabasePostgresAzure,
  },
  {
    dbMeta: { location: DatabaseLocation.Azure, engine: DatabaseEngine.MySql },
    name: 'MySQL',
    keywords: azureKeywords + 'mysql',
    kind: ResourceKind.Database,
    icon: 'Azure',
    unguidedLink: getDbAccessDocLink('azure-postgres-mysql'),
    event: DiscoverEventResource.DatabaseMysqlAzure,
  },
  {
    dbMeta: {
      location: DatabaseLocation.Azure,
      engine: DatabaseEngine.SqlServer,
    },
    name: 'SQL Server (Preview)',
    keywords:
      azureKeywords + 'active directory ad sql server sqlserver preview',
    kind: ResourceKind.Database,
    icon: 'Azure',
    unguidedLink: getDbAccessDocLink('azure-sql-server-ad'),
    event: DiscoverEventResource.DatabaseSqlServerAzure,
  },
  {
    dbMeta: {
      location: DatabaseLocation.Microsoft,
      engine: DatabaseEngine.SqlServer,
    },
    name: 'SQL Server (Preview)',
    keywords:
      baseDatabaseKeywords +
      'microsoft active directory ad sql server sqlserver preview',
    kind: ResourceKind.Database,
    icon: 'Windows',
    unguidedLink: getDbAccessDocLink('sql-server-ad'),
    event: DiscoverEventResource.DatabaseSqlServerMicrosoft,
  },
  {
    dbMeta: { location: DatabaseLocation.Gcp, engine: DatabaseEngine.MySql },
    name: 'Cloud SQL MySQL',
    keywords: gcpKeywords + 'mysql',
    kind: ResourceKind.Database,
    icon: 'Gcp',
    unguidedLink: getDbAccessDocLink('mysql-cloudsql'),
    event: DiscoverEventResource.DatabaseMysqlGcp,
  },
  {
    dbMeta: { location: DatabaseLocation.Gcp, engine: DatabaseEngine.Postgres },
    name: 'Cloud SQL PostgreSQL',
    keywords: gcpKeywords + 'postgresql',
    kind: ResourceKind.Database,
    icon: 'Gcp',
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
    icon: 'Mongo',
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
    icon: 'SelfHosted',
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
    icon: 'Cockroach',
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
    icon: 'SelfHosted',
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
    icon: 'Mongo',
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
    icon: 'SelfHosted',
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
    icon: 'SelfHosted',
    unguidedLink: getDbAccessDocLink('redis-cluster'),
    event: DiscoverEventResource.DatabaseRedisClusterSelfHosted,
  },
  {
    dbMeta: {
      location: DatabaseLocation.TODO,
      engine: DatabaseEngine.Snowflake,
    },
    name: 'Snowflake (Preview)',
    keywords: baseDatabaseKeywords + 'snowflake preview',
    kind: ResourceKind.Database,
    icon: 'Snowflake',
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
    icon: 'Aws',
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
    icon: 'Aws',
    event: DiscoverEventResource.DatabasePostgresRds,
  },
  {
    dbMeta: { location: DatabaseLocation.Aws, engine: DatabaseEngine.MySql },
    name: 'RDS MySQL/MariaDB',
    keywords: awsKeywords + 'rds mysql mariadb',
    kind: ResourceKind.Database,
    icon: 'Aws',
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
    icon: 'Aws',
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
    icon: 'Postgres',
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
    icon: 'SelfHosted',
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
