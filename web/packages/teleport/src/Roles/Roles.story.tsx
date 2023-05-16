/*
Copyright 2019-2021 Gravitational, Inc.
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

import { Roles } from './Roles';

export default {
  title: 'Teleport/Roles',
};

export function Processing() {
  return <Roles {...sample} attempt={{ status: 'processing' as any }} />;
}

export function Loaded() {
  return <Roles {...sample} />;
}

export function Empty() {
  return <Roles {...sample} items={[]} />;
}

export function Failed() {
  return (
    <Roles
      {...sample}
      attempt={{ status: 'failed', statusText: 'some error message' }}
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
      "kind: role\nmetadata:\n  labels:\n    gravitational.io/system: \"true\"\n  name: '@teleadmin'\nspec:\n  allow:\n    kubernetes_groups:\n    - admin\n    logins:\n    - root\n    node_labels:\n      '*': '*'\n    rules:\n    - resources:\n      - '*'\n      verbs:\n      - '*'\n  deny: {}\n  options:\n    cert_format: standard\n    client_idle_timeout: 0s\n    disconnect_expired_cert: false\n    forward_agent: false\n    max_session_ttl: 30h0m0s\n    port_forwarding: true\nversion: v3\n",
  },
  {
    id: 'role:admin',
    kind: 'role' as const,
    name: 'admin',
    displayName: 'admin',
    content:
      "kind: role\nmetadata:\n  name: admin\nspec:\n  allow:\n    kubernetes_groups:\n    - '{{internal.kubernetes_groups}}'\n    logins:\n    - '{{internal.logins}}'\n    - root\n    node_labels:\n      '*': '*'\n    rules:\n    - resources:\n      - role\n      verbs:\n      - list\n      - create\n      - read\n      - update\n      - delete\n    - resources:\n      - auth_connector\n      verbs:\n      - list\n      - create\n      - read\n      - update\n      - delete\n    - resources:\n      - session\n      verbs:\n      - list\n      - read\n    - resources:\n      - trusted_cluster\n      verbs:\n      - list\n      - create\n      - read\n      - update\n      - delete\n  deny: {}\n  options:\n    cert_format: standard\n    client_idle_timeout: 0s\n    disconnect_expired_cert: false\n    forward_agent: true\n    max_session_ttl: 30h0m0s\n    port_forwarding: true\nversion: v3\n",
  },
];

const sample = {
  attempt: {
    status: 'success' as any,
  },
  items: roles,
  remove: () => null,
  save: () => null,
};
