import React from 'react';
import TeleportContext, {
  TeleportContextProvider,
} from 'e-teleport/teleportEContext';

import TrustedClusters from './TrustedClusters';

export default {
  title: 'TeleportE/TrustedClusters',
};

export const Loaded = () => {
  const ctx = new TeleportContext();
  ctx.resourceService.fetchTrustedClusters = () =>
    Promise.resolve(trustedClusters);
  ctx.storeUser.getTrustedClusterAccess = () => acl;

  return render(ctx);
};

export const Failed = () => {
  const ctx = new TeleportContext();
  ctx.resourceService.fetchTrustedClusters = () =>
    Promise.reject(new Error('Failed to load...'));
  ctx.storeUser.getTrustedClusterAccess = () => acl;
  return render(ctx);
};

export const Empty = () => {
  const ctx = new TeleportContext();
  ctx.resourceService.fetchTrustedClusters = () => Promise.resolve([]);
  ctx.storeUser.getTrustedClusterAccess = () => acl;
  return render(ctx);
};

export const CannotCreate = () => {
  const ctx = new TeleportContext();
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

function render(ctx: TeleportContext) {
  return (
    <TeleportContextProvider value={ctx}>
      <TrustedClusters />
    </TeleportContextProvider>
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
    name: 'admin',
    displayName: 'admin',
    content:
      "kind: role\nmetadata:\n  name: admin\nspec:\n  allow:\n    kubernetes_groups:\n    - '{{internal.kubernetes_groups}}'\n    logins:\n    - '{{internal.logins}}'\n    - root\n    node_labels:\n      '*': '*'\n    rules:\n    - resources:\n      - role\n      verbs:\n      - list\n      - create\n      - read\n      - update\n      - delete\n    - resources:\n      - auth_connector\n      verbs:\n      - list\n      - create\n      - read\n      - update\n      - delete\n    - resources:\n      - session\n      verbs:\n      - list\n      - read\n    - resources:\n      - trusted_cluster\n      verbs:\n      - list\n      - create\n      - read\n      - update\n      - delete\n  deny: {}\n  options:\n    cert_format: standard\n    client_idle_timeout: 0s\n    disconnect_expired_cert: false\n    forward_agent: true\n    max_session_ttl: 30h0m0s\n    port_forwarding: true\nversion: v3\n",
  },
];
