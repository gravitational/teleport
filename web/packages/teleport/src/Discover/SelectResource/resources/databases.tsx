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

import { Platform } from 'design/platform';
import { DbProtocol } from 'shared/services/databases';

import { DiscoverEventResource } from 'teleport/services/userEvent';
import { DiscoverGuideId } from 'teleport/services/userPreferences/discoverPreference';

import { SelectResourceSpec } from '.';
import { ResourceKind } from '../../Shared/ResourceKind';
import { DatabaseEngine, DatabaseLocation } from '../types';
import {
  awsDatabaseKeywords,
  azureKeywords,
  baseDatabaseKeywords,
  gcpKeywords,
  selfHostedDatabaseKeywords,
} from './keywords';

// DATABASES_UNGUIDED_DOC are documentations that is not specific
// to one type of database.
export const DATABASES_UNGUIDED_DOC: SelectResourceSpec[] = [
  {
    id: DiscoverGuideId.DatabaseAwsRdsProxyPostgres,
    dbMeta: { location: DatabaseLocation.Aws, engine: DatabaseEngine.Doc },
    name: 'RDS Proxy PostgreSQL',
    keywords: [...awsDatabaseKeywords, 'rds', 'proxy', 'postgresql'],
    kind: ResourceKind.Database,
    icon: 'aws',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-aws-databases/rds-proxy-postgres',
    // TODO(lisa): add a new usage event
    event: DiscoverEventResource.DatabaseDocRdsProxy,
  },
  {
    id: DiscoverGuideId.DatabaseAwsRdsProxySqlServer,
    dbMeta: { location: DatabaseLocation.Aws, engine: DatabaseEngine.Doc },
    name: 'RDS Proxy SQL Server',
    keywords: [
      ...awsDatabaseKeywords,
      'rds',
      'proxy',
      'sql server',
      'sqlserver',
    ],
    kind: ResourceKind.Database,
    icon: 'aws',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-aws-databases/rds-proxy-sqlserver',
    // TODO(lisa): add a new usage event
    event: DiscoverEventResource.DatabaseDocRdsProxy,
  },
  {
    id: DiscoverGuideId.DatabaseAwsRdsProxyMariaMySql,
    dbMeta: { location: DatabaseLocation.Aws, engine: DatabaseEngine.Doc },
    name: 'RDS Proxy MariaDB/MySQL',
    keywords: [...awsDatabaseKeywords, 'rds', 'proxy', 'mariadb', 'mysql'],
    kind: ResourceKind.Database,
    icon: 'aws',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-aws-databases/rds-proxy-mysql',
    // TODO(lisa): add a new usage event
    event: DiscoverEventResource.DatabaseDocRdsProxy,
  },
  {
    id: DiscoverGuideId.DatabaseHighAvailability,
    dbMeta: { location: DatabaseLocation.TODO, engine: DatabaseEngine.Doc },
    name: 'High Availability',
    keywords: [...baseDatabaseKeywords, 'high availability', 'ha'],
    kind: ResourceKind.Database,
    icon: 'database',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/guides/ha',
    event: DiscoverEventResource.DatabaseDocHighAvailability,
  },
  {
    id: DiscoverGuideId.DatabaseDynamicRegistration,
    dbMeta: { location: DatabaseLocation.TODO, engine: DatabaseEngine.Doc },
    name: 'Dynamic Registration',
    keywords: [...baseDatabaseKeywords, 'dynamic registration'],
    kind: ResourceKind.Database,
    icon: 'database',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/guides/dynamic-registration',
    event: DiscoverEventResource.DatabaseDocDynamicRegistration,
  },
];

export const DATABASES_UNGUIDED: SelectResourceSpec[] = [
  {
    id: DiscoverGuideId.DatabaseAwsDynamoDb,
    dbMeta: { location: DatabaseLocation.Aws, engine: DatabaseEngine.DynamoDb },
    name: 'DynamoDB',
    keywords: [...awsDatabaseKeywords, 'dynamodb'],
    kind: ResourceKind.Database,
    icon: 'dynamo',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-aws-databases/aws-dynamodb',
    event: DiscoverEventResource.DatabaseDynamoDb,
  },
  {
    id: DiscoverGuideId.DatabaseAwsElastiCacheMemoryDb,
    dbMeta: { location: DatabaseLocation.Aws, engine: DatabaseEngine.Redis },
    name: 'ElastiCache & MemoryDB',
    keywords: [...awsDatabaseKeywords, 'elasticache', 'memorydb', 'redis'],
    kind: ResourceKind.Database,
    icon: 'aws',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-aws-databases/redis-aws',
    event: DiscoverEventResource.DatabaseRedisElasticache,
  },
  {
    id: DiscoverGuideId.DatabaseAwsCassandraKeyspaces,
    dbMeta: {
      location: DatabaseLocation.Aws,
      engine: DatabaseEngine.Cassandra,
    },
    name: 'Keyspaces (Apache Cassandra)',
    keywords: [...awsDatabaseKeywords, 'keyspaces', 'apache', 'cassandra'],
    kind: ResourceKind.Database,
    icon: 'aws',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-aws-databases/aws-cassandra-keyspaces',
    event: DiscoverEventResource.DatabaseCassandraKeyspaces,
  },
  {
    id: DiscoverGuideId.DatabaseAwsPostgresRedshift,
    dbMeta: { location: DatabaseLocation.Aws, engine: DatabaseEngine.Redshift },
    name: 'Redshift PostgreSQL',
    keywords: [...awsDatabaseKeywords, 'redshift', 'postgresql'],
    kind: ResourceKind.Database,
    icon: 'redshift',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-aws-databases/postgres-redshift',
    event: DiscoverEventResource.DatabasePostgresRedshift,
  },
  {
    id: DiscoverGuideId.DatabaseAwsRedshiftServerless,
    dbMeta: { location: DatabaseLocation.Aws, engine: DatabaseEngine.Redshift },
    name: 'Redshift Serverless',
    keywords: [...awsDatabaseKeywords, 'redshift', 'serverless', 'postgresql'],
    kind: ResourceKind.Database,
    icon: 'redshift',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-aws-databases/redshift-serverless',
    event: DiscoverEventResource.DatabasePostgresRedshiftServerless,
  },
  {
    id: DiscoverGuideId.DatabaseAzureRedis,
    dbMeta: { location: DatabaseLocation.Azure, engine: DatabaseEngine.Redis },
    name: 'Cache for Redis',
    keywords: [...azureKeywords, 'cache', 'redis'],
    kind: ResourceKind.Database,
    icon: 'azure',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-azure-databases/azure-redis',
    event: DiscoverEventResource.DatabaseRedisAzureCache,
  },
  {
    id: DiscoverGuideId.DatabaseAzurePostgres,
    dbMeta: {
      location: DatabaseLocation.Azure,
      engine: DatabaseEngine.Postgres,
    },
    name: 'PostgreSQL',
    keywords: [...azureKeywords, 'postgresql'],
    kind: ResourceKind.Database,
    icon: 'azure',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-azure-databases/azure-postgres-mysql',
    event: DiscoverEventResource.DatabasePostgresAzure,
  },
  {
    id: DiscoverGuideId.DatabaseAzureMysql,
    dbMeta: { location: DatabaseLocation.Azure, engine: DatabaseEngine.MySql },
    name: 'MySQL',
    keywords: [...azureKeywords, 'mysql'],
    kind: ResourceKind.Database,
    icon: 'azure',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-azure-databases/azure-postgres-mysql',
    event: DiscoverEventResource.DatabaseMysqlAzure,
  },
  {
    id: DiscoverGuideId.DatabaseAzureSqlServerAd,
    dbMeta: {
      location: DatabaseLocation.Azure,
      engine: DatabaseEngine.SqlServer,
    },
    name: 'SQL Server',
    keywords: [
      ...azureKeywords,
      'active directory',
      'ad',
      'sql server',
      'sqlserver',
      'preview',
    ],
    kind: ResourceKind.Database,
    icon: 'azure',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-azure-databases/azure-sql-server-ad',
    event: DiscoverEventResource.DatabaseSqlServerAzure,
    platform: Platform.Windows,
  },
  {
    id: DiscoverGuideId.DatabaseAwsSqlServerAd,
    dbMeta: {
      location: DatabaseLocation.Aws,
      engine: DatabaseEngine.SqlServer,
    },
    name: 'RDS SQL Server',
    keywords: [
      ...awsDatabaseKeywords,
      'rds',
      'microsoft',
      'active directory',
      'ad',
      'sql server',
      'sqlserver',
      'preview',
    ],
    kind: ResourceKind.Database,
    icon: 'aws',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-aws-databases/sql-server-ad',
    event: DiscoverEventResource.DatabaseSqlServerMicrosoft,
    platform: Platform.Windows,
  },
  {
    id: DiscoverGuideId.DatabaseGcpMysqlCloudSql,
    dbMeta: { location: DatabaseLocation.Gcp, engine: DatabaseEngine.MySql },
    name: 'Cloud SQL MySQL',
    keywords: [...gcpKeywords, 'mysql'],
    kind: ResourceKind.Database,
    icon: 'googlecloud',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-google-cloud-databases/mysql-cloudsql',
    event: DiscoverEventResource.DatabaseMysqlGcp,
  },
  {
    id: DiscoverGuideId.DatabaseGcpPostgresCloudSql,
    dbMeta: { location: DatabaseLocation.Gcp, engine: DatabaseEngine.Postgres },
    name: 'Cloud SQL PostgreSQL',
    keywords: [...gcpKeywords, 'postgresql'],
    kind: ResourceKind.Database,
    icon: 'googlecloud',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-google-cloud-databases/postgres-cloudsql',
    event: DiscoverEventResource.DatabasePostgresGcp,
  },
  {
    id: DiscoverGuideId.DatabaseMongoAtlas,
    dbMeta: {
      location: DatabaseLocation.TODO,
      engine: DatabaseEngine.MongoDb,
    },
    name: 'MongoDB Atlas',
    keywords: [...baseDatabaseKeywords, 'mongodb atlas'],
    kind: ResourceKind.Database,
    icon: 'mongo',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-managed-databases/mongodb-atlas',
    event: DiscoverEventResource.DatabaseMongodbAtlas,
  },
  {
    id: DiscoverGuideId.DatabaseCassandraScyllaDb,
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.Cassandra,
    },
    name: 'Cassandra & ScyllaDB',
    keywords: [...selfHostedDatabaseKeywords, 'cassandra scylladb'],
    kind: ResourceKind.Database,
    icon: 'selfhosted',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-self-hosted-databases/cassandra-self-hosted',
    event: DiscoverEventResource.DatabaseCassandraSelfHosted,
  },
  {
    id: DiscoverGuideId.DatabaseCockroachDb,
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.CockroachDb,
    },
    name: 'CockroachDB',
    keywords: [...selfHostedDatabaseKeywords, 'cockroachdb'],
    kind: ResourceKind.Database,
    icon: 'cockroach',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-self-hosted-databases/cockroachdb-self-hosted',
    event: DiscoverEventResource.DatabaseCockroachDbSelfHosted,
  },
  {
    id: DiscoverGuideId.DatabaseElasticSearch,
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.ElasticSearch,
    },
    name: 'Elasticsearch',
    keywords: [...selfHostedDatabaseKeywords, 'elasticsearch', 'es'],
    kind: ResourceKind.Database,
    icon: 'selfhosted',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-self-hosted-databases/elastic',
    event: DiscoverEventResource.DatabaseElasticSearchSelfHosted,
  },
  {
    id: DiscoverGuideId.DatabaseMongoDb,
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.MongoDb,
    },
    name: 'MongoDB',
    keywords: [...selfHostedDatabaseKeywords, 'mongodb'],
    kind: ResourceKind.Database,
    icon: 'mongo',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-self-hosted-databases/mongodb-self-hosted',
    event: DiscoverEventResource.DatabaseMongodbSelfHosted,
  },
  {
    id: DiscoverGuideId.DatabaseRedis,
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.Redis,
    },
    name: 'Redis',
    keywords: [...selfHostedDatabaseKeywords, 'redis'],
    kind: ResourceKind.Database,
    icon: 'selfhosted',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-self-hosted-databases/redis',
    event: DiscoverEventResource.DatabaseRedisSelfHosted,
  },
  {
    id: DiscoverGuideId.DatabaseRedisCluster,
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.Redis,
    },
    name: 'Redis Cluster',
    keywords: [...selfHostedDatabaseKeywords, 'redis cluster'],
    kind: ResourceKind.Database,
    icon: 'selfhosted',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-self-hosted-databases/redis-cluster',
    event: DiscoverEventResource.DatabaseRedisClusterSelfHosted,
  },
  {
    id: DiscoverGuideId.DatabaseSnowflake,
    dbMeta: {
      location: DatabaseLocation.TODO,
      engine: DatabaseEngine.Snowflake,
    },
    name: 'Snowflake',
    keywords: [...baseDatabaseKeywords, 'snowflake preview'],
    kind: ResourceKind.Database,
    icon: 'snowflake',
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/database-access/enroll-managed-databases/snowflake',
    event: DiscoverEventResource.DatabaseSnowflake,
  },
];

export const DATABASES: SelectResourceSpec[] = [
  {
    id: DiscoverGuideId.DatabaseAwsRdsPostgres,
    dbMeta: {
      location: DatabaseLocation.Aws,
      engine: DatabaseEngine.Postgres,
    },
    name: 'RDS PostgreSQL',
    keywords: [...awsDatabaseKeywords, 'rds postgresql'],
    kind: ResourceKind.Database,
    icon: 'aws',
    event: DiscoverEventResource.DatabasePostgresRds,
  },
  {
    id: DiscoverGuideId.DatabaseAwsRdsAuroraPostgres,
    dbMeta: {
      location: DatabaseLocation.Aws,
      engine: DatabaseEngine.AuroraPostgres,
    },
    name: 'RDS Aurora PostgreSQL',
    keywords: [...awsDatabaseKeywords, 'rds aurora postgresql'],
    kind: ResourceKind.Database,
    icon: 'aws',
    event: DiscoverEventResource.DatabasePostgresRds,
  },
  {
    id: DiscoverGuideId.DatabaseAwsRdsMysqlMariaDb,
    dbMeta: { location: DatabaseLocation.Aws, engine: DatabaseEngine.MySql },
    name: 'RDS MySQL/MariaDB',
    keywords: [...awsDatabaseKeywords, 'rds mysql mariadb'],
    kind: ResourceKind.Database,
    icon: 'aws',
    event: DiscoverEventResource.DatabaseMysqlRds,
  },
  {
    id: DiscoverGuideId.DatabaseAwsRdsAuroraMysql,
    dbMeta: {
      location: DatabaseLocation.Aws,
      engine: DatabaseEngine.AuroraMysql,
    },
    name: 'RDS Aurora MySQL',
    keywords: [...awsDatabaseKeywords, 'rds aurora mysql'],
    kind: ResourceKind.Database,
    icon: 'aws',
    event: DiscoverEventResource.DatabaseMysqlRds,
  },
  {
    id: DiscoverGuideId.DatabasePostgres,
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.Postgres,
    },
    name: 'PostgreSQL',
    keywords: [...selfHostedDatabaseKeywords, 'postgresql'],
    kind: ResourceKind.Database,
    icon: 'postgres',
    event: DiscoverEventResource.DatabasePostgresSelfHosted,
  },
  {
    id: DiscoverGuideId.DatabaseMysql,
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.MySql,
    },
    name: 'MySQL/MariaDB',
    keywords: [...selfHostedDatabaseKeywords, 'mysql mariadb'],
    kind: ResourceKind.Database,
    icon: 'selfhosted',
    event: DiscoverEventResource.DatabaseMysqlSelfHosted,
  },
];

export function getDatabaseProtocol(engine: DatabaseEngine): DbProtocol {
  switch (engine) {
    case DatabaseEngine.Postgres:
    case DatabaseEngine.AuroraPostgres:
    case DatabaseEngine.Redshift:
      return 'postgres';
    case DatabaseEngine.MySql:
    case DatabaseEngine.AuroraMysql:
      return 'mysql';
    case DatabaseEngine.MongoDb:
      return 'mongodb';
    case DatabaseEngine.Redis:
      return 'redis';
    case DatabaseEngine.CockroachDb:
      return 'cockroachdb';
    case DatabaseEngine.SqlServer:
      return 'sqlserver';
    case DatabaseEngine.Snowflake:
      return 'snowflake';
    case DatabaseEngine.Cassandra:
      return 'cassandra';
    case DatabaseEngine.ElasticSearch:
      return 'elasticsearch';
    case DatabaseEngine.DynamoDb:
      return 'dynamodb';
    case DatabaseEngine.Doc:
      return '' as any;
    default:
      engine satisfies never;
  }
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
