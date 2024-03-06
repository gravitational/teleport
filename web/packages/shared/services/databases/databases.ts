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
  | 'dynamodb'
  | 'clickhouse'
  | 'clickhouse-http';

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
