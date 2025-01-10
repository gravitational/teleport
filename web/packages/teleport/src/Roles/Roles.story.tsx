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

import { useEffect, useRef } from 'react';

import { Roles } from './Roles';

export default {
  title: 'Teleport/Roles',
};

export function Processing() {
  const promiseRef = useRef(Promise.withResolvers<never>());
  useEffect(() => {
    const promise = promiseRef.current;
    return () => promise.resolve(undefined);
  }, []);
  return <Roles {...sample} fetch={() => promiseRef.current.promise} />;
}

export function Loaded() {
  return <Roles {...sample} />;
}

export function Empty() {
  return (
    <Roles {...sample} fetch={async () => ({ items: [], startKey: '' })} />
  );
}

export function Failed() {
  return (
    <Roles
      {...sample}
      fetch={async () => {
        throw new Error('some error message');
      }}
    />
  );
}

const roles = [
  {
    id: 'role:@teleadmin',
    kind: 'role' as const,
    name: '@teleadmin',
    displayName: '@teleadmin',
    content:
      "kind: role\nmetadata:\n  labels:\n    gravitational.io/system: \"true\"\n  name: '@teleadmin'\nspec:\n  allow:\n    kubernetes_groups:\n    - admin\n    logins:\n    - root\n    node_labels:\n      '*': '*'\n    rules:\n    - resources:\n      - '*'\n      verbs:\n      - '*'\n  deny: {}\n  options:\n    cert_format: standard\n    client_idle_timeout: 0s\n    disconnect_expired_cert: false\n    forward_agent: false\n    max_session_ttl: 30h0m0s\n    ssh_port_forwarding:\n      remote:\n        enabled: false\n      local:\n        enabled: false\nversion: v3\n",
  },
  {
    id: 'role:admin',
    kind: 'role' as const,
    name: 'admin',
    displayName: 'admin',
    content:
      "kind: role\nmetadata:\n  name: admin\nspec:\n  allow:\n    kubernetes_groups:\n    - '{{internal.kubernetes_groups}}'\n    logins:\n    - '{{internal.logins}}'\n    - root\n    node_labels:\n      '*': '*'\n    rules:\n    - resources:\n      - role\n      verbs:\n      - list\n      - create\n      - read\n      - update\n      - delete\n    - resources:\n      - auth_connector\n      verbs:\n      - list\n      - create\n      - read\n      - update\n      - delete\n    - resources:\n      - session\n      verbs:\n      - list\n      - read\n    - resources:\n      - trusted_cluster\n      verbs:\n      - list\n      - create\n      - read\n      - update\n      - delete\n  deny: {}\n  options:\n    cert_format: standard\n    client_idle_timeout: 0s\n    disconnect_expired_cert: false\n    forward_agent: true\n    max_session_ttl: 30h0m0s\n    ssh_port_forwarding:\n      remote:\n        enabled: false\n      local:\n        enabled: false\nversion: v3\n",
  },
];

const sample = {
  attempt: {
    status: 'success' as any,
  },
  fetch: async () => ({ items: roles, startKey: '' }),
  remove: () => null,
  create: () => null,
  update: () => null,
  rolesAcl: {
    list: true,
    create: true,
    remove: true,
    edit: true,
    read: true,
  },
};
