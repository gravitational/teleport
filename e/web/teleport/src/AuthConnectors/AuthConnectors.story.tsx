import React from 'react';

import { AuthConnectors } from './AuthConnectors';

export default {
  title: 'TeleportE/AuthConnectors',
};

export function Processing() {
  return (
    <AuthConnectors {...sample} attempt={{ status: 'processing' as any }} />
  );
}

export function Loaded() {
  return <AuthConnectors {...sample} />;
}

export function Empty() {
  return <AuthConnectors {...sample} items={[]} />;
}

export function Failed() {
  return (
    <AuthConnectors
      {...sample}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
  );
}

const connectors = [
  {
    id: 'oidc:googleZufuban',
    kind: 'saml' as const,
    name: 'Okta',
    displayName: 'Okta',
    content:
      "kind: oidc\nmetadata:\n  name: google\nspec:\n  claims_to_roles:\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.com\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.io\n  client_id: 529920086732-v30abileumfve0vhjtasn7l0k5cqt3p7.apps.googleusercontent.com\n  client_secret: k1NZ2WiB0VjVEpf-XInlHkCz\n  display: Google\n  issuer_url: https://accounts.google.com\n  redirect_url: https://demo.gravitational.io:443/portalapi/v1/oidc/callback\n  scope:\n  - email\nversion: v2\n",
  },
  {
    id: 'oidc:googleGogesu',
    kind: 'oidc' as const,
    name: 'google',
    displayName: 'google',
    content:
      "kind: oidc\nmetadata:\n  name: google\nspec:\n  claims_to_roles:\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.com\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.io\n  client_id: 529920086732-v30abileumfve0vhjtasn7l0k5cqt3p7.apps.googleusercontent.com\n  client_secret: k1NZ2WiB0VjVEpf-XInlHkCz\n  display: Google\n  issuer_url: https://accounts.google.com\n  redirect_url: https://demo.gravitational.io:443/portalapi/v1/oidc/callback\n  scope:\n  - email\nversion: v2\n",
  },
  {
    id: 'oidc:googlePetizu',
    kind: 'github' as const,
    name: 'github',
    displayName: 'Github',
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
