import React from 'react';

import { Desktop } from 'teleport/services/desktops';
import { Database } from 'teleport/services/databases';
import { App } from 'teleport/services/apps';
import { Kube } from 'teleport/services/kube';
import { Node } from 'teleport/services/nodes';

import { getEmptyResourceState } from '../useNewRequest';

import { ResourceList, Props } from './ResourceList';

export default {
  title: 'TeleportE/Workflow/ResourceList',
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

const props: Props = {
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
    uri: 'https://console.aws.amazon.com/ec2/v2/home',
    publicAddr: 'awsconsole-1.teleport-proxy.com',
    labels: [
      {
        name: 'aws_account_id',
        value: 'A1234',
      },
    ],
    description: 'This is an AWS Console app',
    awsConsole: true,
    awsRoles: [],
    clusterId: 'one',
    fqdn: 'awsconsole-1.com',
    id: 'one-aws-console-1-awsconsole-1.teleport-proxy.com',
    launchUrl: '',
  },
];

const nodes: Node[] = [
  {
    tunnel: false,
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
    name: 'bb8411a4-ba50-537c-89b3-226a00447bc6',
    addr: 'host.com',
    labels: [{ name: 'foo', value: 'bar' }],
  },
];

const kubes: Kube[] = [
  {
    name: 'tele.logicoma.dev-prod',
    labels: [{ name: 'env', value: 'prod' }],
  },
];
