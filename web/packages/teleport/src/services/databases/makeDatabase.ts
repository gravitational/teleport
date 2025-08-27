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

import { Aws, Database, DatabaseServer, DatabaseService } from './types';

export function makeDatabase(json: any): Database {
  const { name, desc, protocol, type, aws, requiresRequest, targetHealth } =
    json;

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
        securityGroups: aws.rds?.security_groups,
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
    roles: json.database_roles || [],
    hostname: json.hostname,
    aws: madeAws,
    requiresRequest,
    supportsInteractive: json.supports_interactive || false,
    autoUsersEnabled: json.auto_users_enabled || false,
    targetHealth: targetHealth && {
      status: targetHealth.status,
      error: targetHealth.transition_error,
      message: targetHealth.message,
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

export function makeDatabaseServer(json: any): DatabaseServer {
  const { spec, status } = json;

  return {
    hostname: spec?.hostname,
    hostId: spec?.host_id,
    targetHealth: status &&
      status.target_health && {
        status: status.target_health.status,
        message: status.target_health.message,
        error: status.target_health.transition_error,
      },
  };
}
