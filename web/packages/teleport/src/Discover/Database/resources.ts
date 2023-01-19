/**
 * Copyright 2022 Gravitational, Inc.
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

export enum DatabaseLocation {
  AWS,
  SelfHosted,
  GCP,
}

export enum DatabaseEngine {
  PostgreSQL,
  MySQL,
  SQLServer,
  RedShift,
  Mongo,
  Redis,
}

export interface Database {
  location: DatabaseLocation;
  engine: DatabaseEngine;
  name: string;
  popular?: boolean;
}

export const DATABASES: Database[] = [
  {
    location: DatabaseLocation.AWS,
    engine: DatabaseEngine.PostgreSQL,
    name: 'AWS RDS PostgreSQL',
    popular: true,
  },
  {
    location: DatabaseLocation.SelfHosted,
    engine: DatabaseEngine.PostgreSQL,
    name: 'Self-Hosted PostgreSQL',
    popular: true,
  },

  // Unimplemented AWS RDS
  {
    location: DatabaseLocation.AWS,
    engine: DatabaseEngine.MySQL,
    name: 'AWS RDS MySQL',
    popular: false,
  },
  {
    location: DatabaseLocation.AWS,
    engine: DatabaseEngine.SQLServer,
    name: 'AWS RDS SQL Server',
    popular: false,
  },

  // Unimplemented redshift
  {
    location: DatabaseLocation.AWS,
    engine: DatabaseEngine.RedShift,
    name: 'Redshift PostgresSQL',
    popular: false,
  },

  // Unimplemented self-hosted
  {
    location: DatabaseLocation.SelfHosted,
    engine: DatabaseEngine.Mongo,
    name: 'Self-Hosted MongoDB',
    popular: false,
  },
  {
    location: DatabaseLocation.SelfHosted,
    engine: DatabaseEngine.MySQL,
    name: 'Self-Hosted MySQL/MariaDB',
    popular: false,
  },
  {
    location: DatabaseLocation.SelfHosted,
    engine: DatabaseEngine.Redis,
    name: 'Self-Hosted Redis',
    popular: false,
  },
  {
    location: DatabaseLocation.SelfHosted,
    engine: DatabaseEngine.SQLServer,
    name: 'Self-hosted SQL Server',
    popular: false,
  },

  // Unimplemented GCP
  {
    location: DatabaseLocation.GCP,
    engine: DatabaseEngine.PostgreSQL,
    name: 'Cloud PostgresSQL',
    popular: false,
  },
  {
    location: DatabaseLocation.GCP,
    engine: DatabaseEngine.MySQL,
    name: 'Cloud MySQL/MariaDB',
    popular: false,
  },
  {
    location: DatabaseLocation.GCP,
    engine: DatabaseEngine.SQLServer,
    name: 'Cloud SQL Server',
    popular: false,
  },
];

export function getDatabaseProtocol(engine: DatabaseEngine): DbProtocol {
  switch (engine) {
    case DatabaseEngine.PostgreSQL:
    case DatabaseEngine.RedShift:
      return 'postgres';
    case DatabaseEngine.MySQL:
      return 'mysql';
    case DatabaseEngine.Mongo:
      return 'mongodb';
    case DatabaseEngine.SQLServer:
      return 'sqlserver';
    case DatabaseEngine.Redis:
      return 'redis';
  }
}
