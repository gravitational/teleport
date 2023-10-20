/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

export type DbType =
  | 'self-hosted'
  | 'rds'
  | 'rdsproxy'
  | 'redshift'
  | 'redshift-serverless'
  | 'gcp'
  | 'azure'
  | 'elasticache'
  | 'memorydb'
  | 'keyspace'
  | 'cassandra'
  | 'dynamodb'
  | 'opensearch';

export type DbProtocol =
  | 'postgres'
  | 'mysql'
  | 'mongodb'
  | 'oracle'
  | 'redis'
  | 'cockroachdb'
  | 'sqlserver'
  | 'snowflake'
  | 'cassandra'
  | 'elasticsearch'
  | 'opensearch'
  | 'dynamodb';

const formatProtocol = (input: DbProtocol) => {
  switch (input) {
    case 'postgres':
      return 'PostgreSQL';
    case 'mysql':
      return 'MySQL/MariaDB';
    case 'mongodb':
      return 'MongoDB';
    case 'sqlserver':
      return 'SQL Server';
    case 'redis':
      return 'Redis';
    case 'oracle':
      return 'Oracle';
    case 'cockroachdb':
      return 'CockroachDB';
    case 'cassandra':
      return 'Cassandra';
    case 'elasticsearch':
      return 'Elasticsearch';
    default:
      return input;
  }
};

export const formatDatabaseInfo = (type: DbType, protocol: DbProtocol) => {
  const output = { type, protocol, title: '' };

  switch (protocol) {
    case 'snowflake':
      output.title = 'Snowflake';
      return output;
  }
  switch (type) {
    case 'rds':
      output.title = `Amazon RDS ${formatProtocol(protocol)}`;
      return output;
    case 'rdsproxy':
      output.title = `Amazon RDS Proxy ${formatProtocol(protocol)}`;
      return output;
    case 'redshift':
      output.title = 'Amazon Redshift';
      return output;
    case 'redshift-serverless':
      output.title = 'Amazon Redshift Serverless';
      return output;
    case 'elasticache':
      output.title = 'Amazon ElastiCache';
      return output;
    case 'memorydb':
      output.title = 'Amazon MemoryDB';
      return output;
    case 'keyspace':
      output.title = 'Amazon Keyspaces';
      return output;
    case 'dynamodb':
      output.title = 'Amazon DynamoDB';
      return output;
    case 'opensearch':
      output.title = 'Amazon OpenSearch';
      return output;
    case 'self-hosted':
      output.title = `Self-hosted ${formatProtocol(protocol)}`;
      return output;
    case 'gcp':
      output.title = `Cloud SQL ${formatProtocol(protocol)}`;
      return output;
    case 'azure':
      output.title = `Azure ${formatProtocol(protocol)}`;
      return output;
    default:
      output.title = `${type} ${formatProtocol(protocol)}`;
      return output;
  }
};

export type DatabaseInfo = ReturnType<typeof formatDatabaseInfo>;
