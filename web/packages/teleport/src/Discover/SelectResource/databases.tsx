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

import { ResourceKind } from '../Shared/ResourceKind';

import { icons } from './icons';
import { ResourceSpec, DatabaseLocation, DatabaseEngine } from './types';

const baseDatabaseKeywords = 'db database databases';
const awsKeywords = baseDatabaseKeywords + 'aws amazon web services';
const gcpKeywords = baseDatabaseKeywords + 'gcp google cloud provider';
const selfhostedKeywords = baseDatabaseKeywords + 'self hosted self-hosted';
const azureKeywords = baseDatabaseKeywords + 'microsoft azure';

function getDbAccessDocLink(guide: string) {
  return `https://goteleport.com/docs/database-access/guides/${guide}`;
}

export const DATABASES_UNGUIDED: ResourceSpec[] = [
  {
    dbMeta: { location: DatabaseLocation.AWS, engine: DatabaseEngine.TODO },
    name: 'DynamoDB',
    keywords: awsKeywords + 'dynamodb',
    kind: ResourceKind.Database,
    Icon: icons.Dynamo,
    unguidedLink: getDbAccessDocLink('aws-dynamodb'),
  },
  {
    dbMeta: { location: DatabaseLocation.AWS, engine: DatabaseEngine.TODO },
    name: 'ElastiCache & MemoryDB',
    keywords: awsKeywords + 'elasticache memorydb redis',
    kind: ResourceKind.Database,
    Icon: icons.Aws,
    unguidedLink: getDbAccessDocLink('redis-aws'),
  },
  {
    dbMeta: { location: DatabaseLocation.AWS, engine: DatabaseEngine.TODO },
    name: 'Keyspaces (Apache Cassandra)',
    keywords: awsKeywords + 'keyspaces apache cassandra',
    kind: ResourceKind.Database,
    Icon: icons.Aws,
    unguidedLink: getDbAccessDocLink('aws-cassandra-keyspaces'),
  },
  {
    dbMeta: { location: DatabaseLocation.AWS, engine: DatabaseEngine.TODO },
    name: 'RDS & Aurora',
    keywords: awsKeywords + 'rds aurora postgresql mysql mariadb',
    kind: ResourceKind.Database,
    Icon: icons.Aws,
    unguidedLink: getDbAccessDocLink('rds'),
  },
  {
    dbMeta: { location: DatabaseLocation.AWS, engine: DatabaseEngine.TODO },
    name: 'RDS Proxy',
    keywords: awsKeywords + 'rds proxy',
    kind: ResourceKind.Database,
    Icon: icons.Aws,
    unguidedLink: getDbAccessDocLink('rds-proxy'),
  },
  {
    dbMeta: { location: DatabaseLocation.AWS, engine: DatabaseEngine.TODO },
    name: 'Redshift PostgreSQL',
    keywords: awsKeywords + 'redshift postgresql',
    kind: ResourceKind.Database,
    Icon: icons.Redshift,
    unguidedLink: getDbAccessDocLink('postgres-redshift'),
  },
  {
    dbMeta: { location: DatabaseLocation.AWS, engine: DatabaseEngine.TODO },
    name: 'Redshift Serverless',
    keywords: awsKeywords + 'redshift serverless',
    kind: ResourceKind.Database,
    Icon: icons.Redshift,
    unguidedLink: getDbAccessDocLink('redshift-serverless'),
  },
  {
    dbMeta: { location: DatabaseLocation.Azure, engine: DatabaseEngine.TODO },
    name: 'Cache for Redis',
    keywords: azureKeywords + 'cache redis',
    kind: ResourceKind.Database,
    Icon: icons.Azure,
    unguidedLink: getDbAccessDocLink('azure-redis'),
  },
  {
    dbMeta: { location: DatabaseLocation.Azure, engine: DatabaseEngine.TODO },
    name: 'PostgreSQL & MySQL',
    keywords: azureKeywords + 'postgresql mysql',
    kind: ResourceKind.Database,
    Icon: icons.Azure,
    unguidedLink: getDbAccessDocLink('azure-postgres-mysql'),
  },
  {
    dbMeta: { location: DatabaseLocation.Azure, engine: DatabaseEngine.TODO },
    name: 'SQL Server (Preview)',
    keywords: azureKeywords + 'active directory ad sql server preview',
    kind: ResourceKind.Database,
    Icon: icons.Azure,
    unguidedLink: getDbAccessDocLink('azure-sql-server-ad'),
  },
  {
    dbMeta: { location: DatabaseLocation.GCP, engine: DatabaseEngine.TODO },
    name: 'Cloud SQL MySQL',
    keywords: gcpKeywords + 'mysql',
    kind: ResourceKind.Database,
    Icon: icons.Gcp,
    unguidedLink: getDbAccessDocLink('mysql-cloudsql'),
  },
  {
    dbMeta: { location: DatabaseLocation.GCP, engine: DatabaseEngine.TODO },
    name: 'Cloud SQL PostgreSQL',
    keywords: gcpKeywords + 'postgresql',
    kind: ResourceKind.Database,
    Icon: icons.Gcp,
    unguidedLink: getDbAccessDocLink('postgres-cloudsql'),
  },
  {
    dbMeta: { location: DatabaseLocation.Mongo, engine: DatabaseEngine.TODO },
    name: 'MongoDB Atlas',
    keywords: baseDatabaseKeywords + 'mongodb atlas',
    kind: ResourceKind.Database,
    Icon: icons.Mongo,
    unguidedLink: getDbAccessDocLink('mongodb-atlas'),
  },
  {
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.TODO,
    },
    name: 'Cassandra & ScyllaDB',
    keywords: selfhostedKeywords + 'cassandra scylladb',
    kind: ResourceKind.Database,
    Icon: icons.SelfHosted,
    unguidedLink: getDbAccessDocLink('cassandra-self-hosted'),
  },
  {
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.TODO,
    },
    name: 'CockroachDB',
    keywords: selfhostedKeywords + 'cockroachdb',
    kind: ResourceKind.Database,
    Icon: icons.Cockroach,
    unguidedLink: getDbAccessDocLink('cockroachdb-self-hosted'),
  },
  {
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.TODO,
    },
    name: 'Elasticsearch',
    keywords: selfhostedKeywords + 'elasticsearch',
    kind: ResourceKind.Database,
    Icon: icons.SelfHosted,
    unguidedLink: getDbAccessDocLink('elastic'),
  },
  {
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.TODO,
    },
    name: 'MongoDB',
    keywords: selfhostedKeywords + 'mongodb',
    kind: ResourceKind.Database,
    Icon: icons.Mongo,
    unguidedLink: getDbAccessDocLink('mongodb-self-hosted'),
  },
  {
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.TODO,
    },
    name: 'Redis',
    keywords: selfhostedKeywords + 'redis',
    kind: ResourceKind.Database,
    Icon: icons.SelfHosted,
    unguidedLink: getDbAccessDocLink('redis'),
  },
  {
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.TODO,
    },
    name: 'Redis Cluster',
    keywords: selfhostedKeywords + 'redis cluster',
    kind: ResourceKind.Database,
    Icon: icons.SelfHosted,
    unguidedLink: getDbAccessDocLink('redis-cluster'),
  },
  {
    dbMeta: { location: DatabaseLocation.TODO, engine: DatabaseEngine.TODO },
    name: 'Snowflake (Preview)',
    keywords: baseDatabaseKeywords + 'snowflake preview',
    kind: ResourceKind.Database,
    Icon: icons.Snowflake,
    unguidedLink: getDbAccessDocLink('snowflake'),
  },
];

export const DATABASES: ResourceSpec[] = [
  {
    dbMeta: {
      location: DatabaseLocation.AWS,
      engine: DatabaseEngine.PostgreSQL,
    },
    name: 'RDS PostgreSQL',
    keywords: awsKeywords + 'rds postgresql',
    kind: ResourceKind.Database,
    Icon: icons.Aws,
  },
  {
    dbMeta: { location: DatabaseLocation.AWS, engine: DatabaseEngine.MySQL },
    name: 'RDS MySQL/MariaDB',
    keywords: awsKeywords + 'rds mysql mariadb',
    kind: ResourceKind.Database,
    Icon: icons.Aws,
  },
  {
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.PostgreSQL,
    },
    name: 'PostgreSQL',
    keywords: selfhostedKeywords + 'postgresql',
    kind: ResourceKind.Database,
    Icon: icons.Postgres,
  },
  {
    dbMeta: {
      location: DatabaseLocation.SelfHosted,
      engine: DatabaseEngine.MySQL,
    },
    name: 'MySQL/MariaDB',
    keywords: selfhostedKeywords + 'mysql mariadb',
    kind: ResourceKind.Database,
    Icon: icons.SelfHosted,
  },
];

export function getDatabaseProtocol(engine: DatabaseEngine): DbProtocol {
  switch (engine) {
    case DatabaseEngine.PostgreSQL:
      return 'postgres';
    case DatabaseEngine.MySQL:
      return 'mysql';
  }

  return '' as any;
}

export function getDefaultDatabasePort(engine: DatabaseEngine): string {
  switch (engine) {
    case DatabaseEngine.PostgreSQL:
      return '5432';
    case DatabaseEngine.MySQL:
      return '3306';
  }
  return '';
}
