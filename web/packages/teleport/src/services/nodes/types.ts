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

import { NodeSubKind } from 'shared/services';

import { ResourceLabel } from 'teleport/services/agents';

import { Regions } from '../integrations';

export interface Node {
  kind: 'node';
  id: string;
  clusterId: string;
  hostname: string;
  labels: ResourceLabel[];
  addr: string;
  tunnel: boolean;
  subKind: NodeSubKind;
  sshLogins: string[];
  awsMetadata?: AwsMetadata;
}

export interface BashCommand {
  text: string;
  expires: string;
}

export type AwsMetadata = {
  accountId: string;
  instanceId: string;
  region: Regions;
  vpcId: string;
  integration: string;
  subnetId: string;
};

export type CreateNodeRequest = {
  name: string;
  subKind: string;
  hostname: string;
  addr: string;
  labels?: ResourceLabel[];
  aws?: AwsMetadata;
};
