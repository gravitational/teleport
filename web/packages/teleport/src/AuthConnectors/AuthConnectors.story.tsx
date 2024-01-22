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
