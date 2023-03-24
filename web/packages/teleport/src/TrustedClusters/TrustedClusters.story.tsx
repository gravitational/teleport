/*
Copyright 2020 Gravitational, Inc.

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

import React from 'react';

import * as teleport from 'teleport';

import TrustedClusters from './TrustedClusters';

export default {
  title: 'Teleport/TrustedClusters',
};

export const Loaded = () => {
  const ctx = new teleport.Context();
  ctx.resourceService.fetchTrustedClusters = () =>
    Promise.resolve(trustedClusters);
  ctx.storeUser.getTrustedClusterAccess = () => acl;

  return render(ctx);
};

export const Failed = () => {
  const ctx = new teleport.Context();
  ctx.resourceService.fetchTrustedClusters = () =>
    Promise.reject(new Error('Failed to load...'));
  ctx.storeUser.getTrustedClusterAccess = () => acl;
  return render(ctx);
};

export const Empty = () => {
  const ctx = new teleport.Context();
  ctx.resourceService.fetchTrustedClusters = () => Promise.resolve([]);
  ctx.storeUser.getTrustedClusterAccess = () => acl;
  return render(ctx);
};

export const CannotCreate = () => {
  const ctx = new teleport.Context();
  ctx.resourceService.fetchTrustedClusters = () => Promise.resolve([]);
  ctx.storeUser.getTrustedClusterAccess = () => ({ ...acl, create: false });
  return render(ctx);
};

const acl = {
  list: true,
  read: true,
  edit: true,
  create: true,
  remove: true,
};

function render(ctx: teleport.Context) {
  return (
    <teleport.ContextProvider ctx={ctx}>
      <TrustedClusters />
    </teleport.ContextProvider>
  );
}

const trustedClusters = [
  {
    id: 'role:@teleadmin',
    kind: 'trusted_cluster' as const,
    name: '@teleadmin',
    displayName: '@teleadmin',
    content:
      "kind: role\nmetadata:\n  labels:\n    gravitational.io/system: \"true\"\n  name: '@teleadmin'\nspec:\n  allow:\n    kubernetes_groups:\n    - admin\n    logins:\n    - root\n    node_labels:\n      '*': '*'\n    rules:\n    - resources:\n      - '*'\n      verbs:\n      - '*'\n  deny: {}\n  options:\n    cert_format: standard\n    client_idle_timeout: 0s\n    disconnect_expired_cert: false\n    forward_agent: false\n    max_session_ttl: 30h0m0s\n    port_forwarding: true\nversion: v3\n",
  },
  {
    id: 'role:admin',
    kind: 'trusted_cluster' as const,
    name: 'georgewashington.gravitational.io',
    displayName: 'georgewashington.gravitational.io',
    content:
      "kind: role\nmetadata:\n  name: admin\nspec:\n  allow:\n    kubernetes_groups:\n    - '{{internal.kubernetes_groups}}'\n    logins:\n    - '{{internal.logins}}'\n    - root\n    node_labels:\n      '*': '*'\n    rules:\n    - resources:\n      - role\n      verbs:\n      - list\n      - create\n      - read\n      - update\n      - delete\n    - resources:\n      - auth_connector\n      verbs:\n      - list\n      - create\n      - read\n      - update\n      - delete\n    - resources:\n      - session\n      verbs:\n      - list\n      - read\n    - resources:\n      - trusted_cluster\n      verbs:\n      - list\n      - create\n      - read\n      - update\n      - delete\n  deny: {}\n  options:\n    cert_format: standard\n    client_idle_timeout: 0s\n    disconnect_expired_cert: false\n    forward_agent: true\n    max_session_ttl: 30h0m0s\n    port_forwarding: true\nversion: v3\n",
  },
];
