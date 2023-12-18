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

import { formatDatabaseInfo } from 'shared/services/databases';

import { Aws, Database, DatabaseService } from './types';

export function makeDatabase(json: any): Database {
  const { name, desc, protocol, type, aws } = json;

  const labels = json.labels || [];

  // The backend will return the field `aws` as undefined
  // if this database is not hosted by AWS.
  // (Only setting RDS fields for now.)
  let madeAws: Aws;
  if (aws) {
    madeAws = {
      rds: {
        resourceId: aws.rds?.resource_id,
        region: aws.rds?.region,
        vpcId: aws.rds?.vpc_id,
        subnets: aws.rds?.subnets || [],
      },
      iamPolicyStatus: aws.iam_policy_status,
    };
  }

  return {
    kind: 'db',
    name,
    description: desc,
    type: formatDatabaseInfo(type, protocol).title,
    protocol,
    labels,
    names: json.database_names || [],
    users: json.database_users || [],
    hostname: json.hostname,
    aws: madeAws,
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
