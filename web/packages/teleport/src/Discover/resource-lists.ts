/*
Copyright 2022 Gravitational, Inc.

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

export type ResourceLocation = 'AWS' | 'Self-Hosted';

export type ResourceType = {
  name: string;
  key: string;
  type: ResourceLocation;
  popular: boolean;
};

export const resourceTypes: ResourceType[] = [
  {
    name: 'AWS RDS PostgreSQL',
    key: 'rds-postgres',
    type: 'AWS',
    popular: true,
  },
  {
    name: 'AWS RDS MySQL/MariaDB',
    key: 'rds-mysql',
    type: 'AWS',
    popular: true,
  },
  {
    name: 'AWS RDS SQL Server',
    key: 'rds-sql-server',
    type: 'AWS',
    popular: true,
  },
  {
    name: 'AWS Redshift',
    key: 'rds-redshift',
    type: 'AWS',
    popular: false,
  },
  {
    name: 'Self-Hosted PostgreSQL',
    key: 'self-postgres',
    type: 'Self-Hosted',
    popular: false,
  },
  {
    name: 'Self-Hosted MySQL/MariaDB',
    key: 'self-mysql',
    type: 'Self-Hosted',
    popular: false,
  },
  {
    name: 'Self-Hosted MongoDB',
    key: 'self-mongo',
    type: 'Self-Hosted',
    popular: false,
  },
  {
    name: 'Self-Hosted SQL Server',
    key: 'self-sql-server',
    type: 'Self-Hosted',
    popular: false,
  },
  {
    name: 'Self-Hosted Redis',
    key: 'self-redis',
    type: 'Self-Hosted',
    popular: false,
  },
];
