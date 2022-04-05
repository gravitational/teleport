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

import { Database, DbType, DbProtocol } from './types';

export default function makeDatabase(json): Database {
  const { name, desc, protocol, type } = json;

  const labels = json.labels || [];

  return {
    name,
    desc,
    title: formatDatabaseInfo(type, protocol).title,
    protocol,
    tags: labels.map(label => `${label.name}: ${label.value}`),
  };
}

export const formatDatabaseInfo = (type: DbType, protocol: DbProtocol) => {
  const output = { type, protocol, title: '' };

  switch (type) {
    case 'rds':
      output.title = `RDS ${formatProtocol(protocol)}`;
      return output;
    case 'redshift':
      output.title = 'Redshift';
      return output;
    case 'self-hosted':
      output.title = `Self-hosted ${formatProtocol(protocol)}`;
      return output;
    case 'gcp':
      output.title = `Cloud SQL ${formatProtocol(protocol)}`;
      return output;
    default:
      output.title = `${type} ${formatProtocol(protocol)}`;
      return output;
  }
};

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
    default:
      return input;
  }
};

export type DatabaseInfo = ReturnType<typeof formatDatabaseInfo>;
