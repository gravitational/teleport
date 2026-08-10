import { delay, http, HttpResponse } from 'msw';
import { useEffect, type JSX } from 'react';

import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { TeleportProviderBasicE } from 'e-teleport/mocks/providers';
import { connectors } from 'teleport/AuthConnectors/fixtures';

import { AuthConnectors } from './AuthConnectors';

export default {
  title: 'TeleportE/AuthConnectors',
};

export function Loaded() {
  return (
    <ContextWrapper>
      <AuthConnectors />
    </ContextWrapper>
  );
}
Loaded.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.getAuthConnectorsListUrl(), () =>
      HttpResponse.json({ connectors })
    )
  );
};

export function Empty() {
  return (
    <ContextWrapper>
      <AuthConnectors />
    </ContextWrapper>
  );
}
Empty.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.getAuthConnectorsListUrl(), () => HttpResponse.json([]))
  );
};

export function Processing() {
  return (
    <ContextWrapper>
      <AuthConnectors />
    </ContextWrapper>
  );
}
Processing.beforeEach = ({ msw }) => {
  msw.use(
    http.get(
      cfg.getAuthConnectorsListUrl(),
      async () => await delay('infinite')
    )
  );
};

export function Failed() {
  return (
    <ContextWrapper>
      <AuthConnectors />
    </ContextWrapper>
  );
}
Failed.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.getAuthConnectorsListUrl(), () =>
      HttpResponse.json(
        { message: 'something went wrong' },
        {
          status: 500,
        }
      )
    )
  );
};

function ContextWrapper({ children }: { children: JSX.Element }) {
  const ctx = createTeleportContextE();

  useEffect(() => {
    const initialLockedFeatures = ctx.lockedFeatures;
    ctx.lockedFeatures.authConnectors = false;
    return () => {
      ctx.lockedFeatures = initialLockedFeatures;
    };
  });
  return (
    <TeleportProviderBasicE
      teleportCtx={ctx}
      initialEntries={[cfg.oss.routes.sso]}
    >
      {children}
    </TeleportProviderBasicE>
  );
}
