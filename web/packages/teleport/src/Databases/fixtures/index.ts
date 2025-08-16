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

import { Database } from 'teleport/services/databases';

export const databases: Database[] = [
  {
    kind: 'db',
    name: 'aurora',
    description: 'PostgreSQL 11.6: AWS Aurora ',
    type: 'RDS PostgreSQL',
    protocol: 'postgres',
    labels: [
      { name: 'cluster', value: 'root' },
      { name: 'env', value: 'aws' },
    ],
    hostname: 'aurora-hostname',
    targetHealth: { status: 'unhealthy' },
  },
  {
    kind: 'db',
    name: 'mongodbizzle',
    description: 'MongoDB database here',
    type: 'Self-hosted MongoDB',
    protocol: 'mongodb',
    labels: [
      { name: 'cluster', value: 'root' },
      { name: 'env', value: 'aws' },
    ],
    hostname: 'mongo-bongo',
    targetHealth: { status: 'unknown' },
  },
  {
    kind: 'db',
    name: 'Dynamooooo',
    description: 'AWS Dynamo',
    type: 'AWS Dynamo',
    protocol: 'dynamodb',
    labels: [
      { name: 'cluster', value: 'root' },
      { name: 'env', value: 'aws' },
    ],
    hostname: 'dynamo-123',
  },
  {
    kind: 'db',
    name: 'Cassandra 45',
    description: 'The Cassandra DB',
    type: 'Cassandra',
    protocol: 'cassandra',
    labels: [
      { name: 'cluster', value: 'root' },
      { name: 'env', value: 'aws' },
    ],
    hostname: 'cas-123',
  },
  {
    kind: 'db',
    name: 'snowyboi',
    description: 'Snowflake',
    type: 'Snowflake',
    protocol: 'snowflake',
    labels: [
      { name: 'cluster', value: 'root' },
      { name: 'env', value: 'aws' },
    ],
    hostname: 'snowflake-stuff',
  },
  {
    kind: 'db',
    name: 'roach',
    description: 'Cockroach DB',
    type: 'Self-hosted CockroachDB',
    protocol: 'cockroachdb',
    labels: [
      { name: 'cluster', value: 'root' },
      { name: 'env', value: 'aws' },
    ],
    hostname: 'cockroach-host',
  },
  {
    kind: 'db',
    name: 'postgres-gcp',
    description: 'PostgreSQL 9.6: Google Cloud SQL',
    type: 'Cloud SQL PostgreSQL',
    protocol: 'postgres',
    labels: [
      { name: 'cluster', value: 'root' },
      { name: 'env', value: 'gcp' },
    ],
    hostname: 'postgres-hostname',
    targetHealth: { status: 'unhealthy' },
  },
  {
    kind: 'db',
    name: 'mysql-aurora-56',
    description: 'MySQL 5.6: AWS Aurora Longname For SQL',
    type: 'Self-hosted MySQL',
    protocol: 'mysql',
    labels: [
      { name: 'cluster', value: 'root' },
      { name: 'env', value: 'aws' },
    ],
    hostname: 'mysql-hostname',
  },
];

export const moreDatabases: Database[] = [
  {
    kind: 'db',
    name: 'Puopte',
    description: 'PostgreSQL 11.6: AWS Aurora ',
    type: 'RDS PostgreSQL',
    protocol: 'postgres',
    labels: [
      { name: 'cluster', value: 'root' },
      { name: 'env', value: 'aws' },
    ],
    hostname: 'aurora-hostname',
  },
  {
    kind: 'db',
    name: 'Sujerej',
    description: 'MongoDB database here',
    type: 'Self-hosted MongoDB',
    protocol: 'mongodb',
    labels: [
      { name: 'cluster', value: 'root' },
      { name: 'env', value: 'aws' },
    ],
    hostname: 'mongo-bongo',
  },
  {
    kind: 'db',
    name: 'Zacocmo',
    description: 'AWS Dynamo',
    type: 'AWS Dynamo',
    protocol: 'dynamodb',
    labels: [
      { name: 'cluster', value: 'root' },
      { name: 'env', value: 'aws' },
    ],
    hostname: 'dynamo-123',
  },
  {
    kind: 'db',
    name: 'Capaede',
    description: 'The Cassandra DB',
    type: 'Cassandra',
    protocol: 'cassandra',
    labels: [
      { name: 'cluster', value: 'root' },
      { name: 'env', value: 'aws' },
    ],
    hostname: 'cas-123',
  },
  {
    kind: 'db',
    name: 'Reirwoc',
    description: 'Snowflake',
    type: 'Snowflake',
    protocol: 'snowflake',
    labels: [
      { name: 'cluster', value: 'root' },
      { name: 'env', value: 'aws' },
    ],
    hostname: 'snowflake-stuff',
  },
  {
    kind: 'db',
    name: 'Rowepjez',
    description: 'Cockroach DB',
    type: 'Self-hosted CockroachDB',
    protocol: 'cockroachdb',
    labels: [
      { name: 'cluster', value: 'root' },
      { name: 'env', value: 'aws' },
    ],
    hostname: 'cockroach-host',
  },
  {
    kind: 'db',
    name: 'Sezago',
    description: 'PostgreSQL 9.6: Google Cloud SQL',
    type: 'Cloud SQL PostgreSQL',
    protocol: 'postgres',
    labels: [
      { name: 'cluster', value: 'root' },
      { name: 'env', value: 'gcp' },
    ],
    hostname: 'postgres-hostname',
  },
  {
    kind: 'db',
    name: 'Bepodo',
    description: 'MySQL 5.6: AWS Aurora Longname For SQL',
    type: 'Self-hosted MySQL',
    protocol: 'mysql',
    labels: [
      { name: 'cluster', value: 'root' },
      { name: 'env', value: 'aws' },
    ],
    hostname: 'mysql-hostname',
  },
];
