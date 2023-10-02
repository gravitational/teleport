/*
Copyright 2020-2021 Gravitational, Inc.
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

import { ContextProvider } from 'teleport';
import { createTeleportContext } from 'teleport/mocks/contexts';

import { AuthConnectors } from './AuthConnectors';

export default {
  title: 'Teleport/AuthConnectors',
};

export function Processing() {
  return (
    <ContextWrapper>
      <AuthConnectors {...sample} attempt={{ status: 'processing' as any }} />
    </ContextWrapper>
  );
}

export function Loaded() {
  return (
    <ContextWrapper>
      <AuthConnectors {...sample} />
    </ContextWrapper>
  );
}

export function Empty() {
  return (
    <ContextWrapper>
      <AuthConnectors {...sample} items={[]} />
    </ContextWrapper>
  );
}

export function Failed() {
  return (
    <ContextWrapper>
      <AuthConnectors
        {...sample}
        attempt={{ status: 'failed', statusText: 'some error message' }}
      />
    </ContextWrapper>
  );
}

function ContextWrapper({ children }: { children: JSX.Element }) {
  const ctx = createTeleportContext();
  return <ContextProvider ctx={ctx}>{children}</ContextProvider>;
}

const connectors = [
  {
    id: 'github:github',
    kind: 'github' as const,
    name: 'github',
    content:
      "kind: oidc\nmetadata:\n  name: google\nspec:\n  claims_to_roles:\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.com\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.io\n  client_id: 529920086732-v30abileumfve0vhjtasn7l0k5cqt3p7.apps.googleusercontent.com\n  client_secret: k1NZ2WiB0VjVEpf-XInlHkCz\n  display: Google\n  issuer_url: https://accounts.google.com\n  redirect_url: https://demo.gravitational.io:443/portalapi/v1/oidc/callback\n  scope:\n  - email\nversion: v2\n",
  },
  {
    id: 'github:github2',
    kind: 'github' as const,
    name: 'github2',
    content:
      "kind: oidc\nmetadata:\n  name: google\nspec:\n  claims_to_roles:\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.com\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.io\n  client_id: 529920086732-v30abileumfve0vhjtasn7l0k5cqt3p7.apps.googleusercontent.com\n  client_secret: k1NZ2WiB0VjVEpf-XInlHkCz\n  display: Google\n  issuer_url: https://accounts.google.com\n  redirect_url: https://demo.gravitational.io:443/portalapi/v1/oidc/callback\n  scope:\n  - email\nversion: v2\n",
  },
];

const sample = {
  attempt: {
    status: 'success' as any,
  },
  items: connectors,
  remove: () => null,
  save: () => null,
};
