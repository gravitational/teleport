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

import { AwsMetadata, Node, SSHLogin } from './types';

export default function makeNode(json: any): Node {
  json = json ?? {};
  const {
    id,
    siteId,
    subKind,
    hostname,
    addr,
    tunnel,
    tags,
    sshLogins,
    sshLoginDetails,
    aws,
    requiresRequest,
    supportedFeatureIds,
  } = json;

  return {
    kind: 'node',
    id,
    subKind,
    clusterId: siteId,
    hostname,
    labels: tags ?? [],
    addr,
    tunnel,
    requiresRequest,
    sshLogins: makeSSHLoginStrings(sshLogins),
    sshLoginDetails: makeSSHLoginDetails(sshLoginDetails),
    awsMetadata: aws ? makeAwsMetadata(aws) : undefined,
    supportedFeatureIds,
  };
}

function makeSSHLoginStrings(raw: any): string[] {
  if (!Array.isArray(raw)) {
    return [];
  }
  return raw.map((item: any) => (typeof item === 'string' ? item : String(item)));
}

function makeSSHLoginDetails(raw: any): SSHLogin[] | undefined {
  if (!Array.isArray(raw)) {
    return undefined;
  }
  return raw.map((item: any) => ({
    login: item.login ?? '',
    requiresRequest: item.requiresRequest,
  }));
}

function makeAwsMetadata(json: any): AwsMetadata {
  json = json ?? {};
  const { accountId, instanceId, region, vpcId, integration, subnetId } = json;

  return {
    accountId,
    instanceId,
    region,
    vpcId,
    integration,
    subnetId,
  };
}
