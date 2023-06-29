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

import { formatDatabaseInfo } from 'shared/services/databases';

import { Database, DatabaseService } from './types';

export function makeDatabase(json: any): Database {
  const { name, desc, protocol, type, aws } = json;

  const labels = json.labels || [];

  return {
    name,
    description: desc,
    type: formatDatabaseInfo(type, protocol).title,
    protocol,
    labels,
    names: json.database_names || [],
    users: json.database_users || [],
    hostname: json.hostname,
    aws: {
      rds: {
        resourceId: aws?.rds?.resource_id,
        region: aws?.rds?.region,
        subnets: aws?.rds?.subnets || [],
      },
    },
  };
}

export function makeDatabaseService(json: any): DatabaseService {
  const { name, resource_matchers } = json;

  return {
    name,
    matcherLabels: combineResourceMatcherLabels(resource_matchers || []),
  };
}

function combineResourceMatcherLabels(
  resourceMatchers: any[]
): Record<string, string[]> {
  const labelMap: Record<string, string[]> = {};

  resourceMatchers.forEach(rm => {
    Object.keys(rm.labels || []).forEach(key => {
      if (!labelMap[key]) {
        labelMap[key] = [];
      }

      // The type return can be a list of strings, or
      // just a string. We convert it to an array
      // to keep it consistent.
      let vals = rm.labels[key];
      if (!Array.isArray(vals)) {
        vals = [vals];
      }

      labelMap[key] = [...labelMap[key], ...vals];
    });
  });

  return labelMap;
}
