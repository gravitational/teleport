/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
