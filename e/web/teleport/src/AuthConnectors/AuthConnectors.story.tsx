import { ContextProvider } from 'teleport';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import { AuthConnectors } from './AuthConnectors';

export default {
  title: 'TeleportE/AuthConnectors',
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

export function EmptyWithCTA() {
  return (
    <ContextWrapper>
      <AuthConnectors {...sample} items={[]} showAuthConnectorsCTA={true} />
    </ContextWrapper>
  );
}

export function LoadedWithCTA() {
  return (
    <ContextWrapper>
      <AuthConnectors {...sample} showAuthConnectorsCTA={true} />
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
  const ctx = createTeleportContextE();
  return <ContextProvider ctx={ctx}>{children}</ContextProvider>;
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
  showAuthConnectorsCTA: false,
};
