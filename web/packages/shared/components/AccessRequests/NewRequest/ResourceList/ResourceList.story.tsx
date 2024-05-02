/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import React from 'react';

import { Desktop } from 'teleport/services/desktops';
import { Database } from 'teleport/services/databases';
import { App } from 'teleport/services/apps';
import { Kube } from 'teleport/services/kube';
import { Node } from 'teleport/services/nodes';
import { UserGroup } from 'teleport/services/userGroups';

import { getEmptyResourceState } from '../resource';

import { ResourceList, ResourceListProps } from './ResourceList';

export default {
  title: 'Shared/AccessRequests/ResourceList',
};

export const Apps = () => <ResourceList {...props} agents={apps} />;

export const Databases = () => (
  <ResourceList {...props} agents={dbs} selectedResource="db" />
);

export const Desktops = () => (
  <ResourceList
    {...props}
    agents={desktops}
    selectedResource="windows_desktop"
  />
);

export const Kubes = () => (
  <ResourceList {...props} agents={kubes} selectedResource="kube_cluster" />
);

export const Nodes = () => (
  <ResourceList {...props} agents={nodes} selectedResource="node" />
);

export const Roles = () => (
  <ResourceList
    {...props}
    requestableRoles={['role1', 'role2']}
    selectedResource="role"
  />
);

export const UserGroups = () => (
  <ResourceList {...props} agents={userGroups} selectedResource="user_group" />
);

const props: ResourceListProps = {
  agents: [],
  selectedResource: 'app',
  customSort: { dir: 'ASC', fieldName: '', onSort: () => null },
  onLabelClick: () => null,
  addedResources: getEmptyResourceState(),
  addOrRemoveResource: () => null,
  requestableRoles: [],
  disableRows: false,
};

const apps: App[] = [
  {
    name: 'aws-console-1',
    kind: 'app',
    uri: 'https://console.aws.amazon.com/ec2/v2/home',
    publicAddr: 'awsconsole-1.teleport-proxy.com',
    addrWithProtocol: 'https://awsconsole-1.teleport-proxy.com',
    labels: [
      {
        name: 'aws_account_id',
        value: 'A1234',
      },
    ],
    description: 'This is an AWS Console app',
    awsConsole: true,
    samlApp: false,
    awsRoles: [],
    clusterId: 'one',
    fqdn: 'awsconsole-1.com',
    id: 'one-aws-console-1-awsconsole-1.teleport-proxy.com',
    launchUrl: '',
    userGroups: [],
  },
];

const nodes: Node[] = [
  {
    tunnel: false,
    kind: 'node',
    subKind: 'teleport',
    sshLogins: ['dev', 'root'],
    id: '104',
    clusterId: 'one',
    hostname: 'fujedu',
    addr: '172.10.1.20:3022',
    labels: [
      {
        name: 'cluster',
        value: 'one',
      },
    ],
  },
];

const dbs: Database[] = [
  {
    name: 'aurora',
    kind: 'db',
    description: 'PostgreSQL 11.6: AWS Aurora ',
    hostname: 'aurora-hostname',
    type: 'RDS PostgreSQL',
    protocol: 'postgres',
    labels: [{ name: 'cluster', value: 'root' }],
  },
];

const desktops: Desktop[] = [
  {
    os: 'windows',
    kind: 'windows_desktop',
    name: 'bb8411a4-ba50-537c-89b3-226a00447bc6',
    addr: 'host.com',
    labels: [{ name: 'foo', value: 'bar' }],
    logins: ['Administrator'],
  },
];

const kubes: Kube[] = [
  {
    name: 'tele.logicoma.dev-prod',
    kind: 'kube_cluster',
    labels: [{ name: 'env', value: 'prod' }],
  },
];

const userGroups: UserGroup[] = [
  {
    kind: 'user_group',
    name: 'group id 1',
    description: 'user group',
    labels: [{ name: 'env', value: 'prod' }],
  },
  {
    kind: 'user_group',
    name: 'group id 2',
    description: 'admin group',
    labels: [{ name: 'env', value: 'dev' }],
  },
];
