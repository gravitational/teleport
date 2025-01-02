import { useEffect, useState } from 'react';

import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport';

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
  const [, setState] = useState({});

  useEffect(() => {
    // Mock enable this to show Entra ID setup tile.
    const defaultIdentityCfg = structuredClone(cfg.oss.entitlements.Identity);
    cfg.oss.entitlements.Identity.enabled = true;
    setState({}); // Force re-render the component with updated cfg.

    return () => {
      cfg.oss.entitlements.Identity = defaultIdentityCfg;
    };
  }, []);

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
    id: 'github:github',
    kind: 'github' as const,
    name: 'Github',
    content:
      "kind: oidc\nmetadata:\n  name: google\nspec:\n  claims_to_roles:\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.com\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.io\n  client_id: 529920086732-v30abileumfve0vhjtasn7l0k5cqt3p7.apps.googleusercontent.com\n  client_secret: k1NZ2WiB0VjVEpf-XInlHkCz\n  display: Google\n  issuer_url: https://accounts.google.com\n  redirect_url: https://demo.gravitational.io:443/portalapi/v1/oidc/callback\n  scope:\n  - email\nversion: v2\n",
  },
  {
    id: 'saml:saml',
    kind: 'saml' as const,
    name: 'Keycloak',
    content:
      "kind: oidc\nmetadata:\n  name: google\nspec:\n  claims_to_roles:\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.com\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.io\n  client_id: 529920086732-v30abileumfve0vhjtasn7l0k5cqt3p7.apps.googleusercontent.com\n  client_secret: k1NZ2WiB0VjVEpf-XInlHkCz\n  display: Google\n  issuer_url: https://accounts.google.com\n  redirect_url: https://demo.gravitational.io:443/portalapi/v1/oidc/callback\n  scope:\n  - email\nversion: v2\n",
  },
  {
    id: 'oidc:oidc',
    kind: 'oidc' as const,
    name: 'My OIDC Connector',
    content:
      "kind: oidc\nmetadata:\n  name: google\nspec:\n  claims_to_roles:\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.com\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.io\n  client_id: 529920086732-v30abileumfve0vhjtasn7l0k5cqt3p7.apps.googleusercontent.com\n  client_secret: k1NZ2WiB0VjVEpf-XInlHkCz\n  display: Google\n  issuer_url: https://accounts.google.com\n  redirect_url: https://demo.gravitational.io:443/portalapi/v1/oidc/callback\n  scope:\n  - email\nversion: v2\n",
  },
  {
    id: 'oidc:google',
    kind: 'oidc' as const,
    name: 'Google',
    content:
      "kind: oidc\nmetadata:\n  name: google\nspec:\n  claims_to_roles:\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.com\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.io\n  client_id: 529920086732-v30abileumfve0vhjtasn7l0k5cqt3p7.apps.googleusercontent.com\n  client_secret: k1NZ2WiB0VjVEpf-XInlHkCz\n  display: Google\n  issuer_url: https://accounts.google.com\n  redirect_url: https://demo.gravitational.io:443/portalapi/v1/oidc/callback\n  scope:\n  - email\nversion: v2\n",
  },
  {
    id: 'saml:okta',
    kind: 'saml' as const,
    name: 'Okta',
    content:
      "kind: oidc\nmetadata:\n  name: google\nspec:\n  claims_to_roles:\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.com\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.io\n  client_id: 529920086732-v30abileumfve0vhjtasn7l0k5cqt3p7.apps.googleusercontent.com\n  client_secret: k1NZ2WiB0VjVEpf-XInlHkCz\n  display: Google\n  issuer_url: https://accounts.google.com\n  redirect_url: https://demo.gravitational.io:443/portalapi/v1/oidc/callback\n  scope:\n  - email\nversion: v2\n",
  },
  {
    id: 'saml:entra',
    kind: 'saml' as const,
    name: 'Microsoft Entra ID',
    content:
      "kind: oidc\nmetadata:\n  name: google\nspec:\n  claims_to_roles:\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.com\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.io\n  client_id: 529920086732-v30abileumfve0vhjtasn7l0k5cqt3p7.apps.googleusercontent.com\n  client_secret: k1NZ2WiB0VjVEpf-XInlHkCz\n  display: Google\n  issuer_url: https://accounts.google.com\n  redirect_url: https://demo.gravitational.io:443/portalapi/v1/oidc/callback\n  scope:\n  - email\nversion: v2\n",
  },
  {
    id: 'saml:onelogin',
    kind: 'saml' as const,
    name: 'OneLogin',
    content:
      "kind: oidc\nmetadata:\n  name: google\nspec:\n  claims_to_roles:\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.com\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.io\n  client_id: 529920086732-v30abileumfve0vhjtasn7l0k5cqt3p7.apps.googleusercontent.com\n  client_secret: k1NZ2WiB0VjVEpf-XInlHkCz\n  display: Google\n  issuer_url: https://accounts.google.com\n  redirect_url: https://demo.gravitational.io:443/portalapi/v1/oidc/callback\n  scope:\n  - email\nversion: v2\n",
  },
  {
    id: 'oidc:gitlab',
    kind: 'saml' as const,
    name: 'GitLab',
    content:
      "kind: oidc\nmetadata:\n  name: google\nspec:\n  claims_to_roles:\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.com\n  - claim: hd\n    roles:\n    - '@teleadmin'\n    value: gravitational.io\n  client_id: 529920086732-v30abileumfve0vhjtasn7l0k5cqt3p7.apps.googleusercontent.com\n  client_secret: k1NZ2WiB0VjVEpf-XInlHkCz\n  display: Google\n  issuer_url: https://accounts.google.com\n  redirect_url: https://demo.gravitational.io:443/portalapi/v1/oidc/callback\n  scope:\n  - email\nversion: v2\n",
  },
  {
    id: 'saml:auth0',
    kind: 'saml' as const,
    name: 'Auth0',
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
