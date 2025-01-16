import { delay, http, HttpResponse } from 'msw';
import { useEffect } from 'react';
import { MemoryRouter } from 'react-router';

import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport';
import { connectors } from 'teleport/AuthConnectors/fixtures';

import { AuthConnectors } from './AuthConnectors';

export default {
  title: 'TeleportE/AuthConnectors',
};

export function Processing() {
  return (
    <MemoryRouter initialEntries={[cfg.oss.routes.sso]}>
      <ContextWrapper>
        <AuthConnectors />
      </ContextWrapper>
    </MemoryRouter>
  );
}
Processing.parameters = {
  msw: {
    handlers: [
      http.get(
        cfg.getAuthConnectorsListUrl(),
        async () => await delay('infinite')
      ),
    ],
  },
};

export function Loaded() {
  return (
    <MemoryRouter initialEntries={[cfg.oss.routes.sso]}>
      <ContextWrapper>
        <AuthConnectors />
      </ContextWrapper>
    </MemoryRouter>
  );
}
Loaded.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getAuthConnectorsListUrl(), () =>
        HttpResponse.json(connectors)
      ),
    ],
  },
};

export function Empty() {
  return (
    <MemoryRouter initialEntries={[cfg.oss.routes.sso]}>
      <ContextWrapper>
        <AuthConnectors />
      </ContextWrapper>
    </MemoryRouter>
  );
}
Empty.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getAuthConnectorsListUrl(), () => HttpResponse.json([])),
    ],
  },
};

export function Failed() {
  return (
    <MemoryRouter initialEntries={[cfg.oss.routes.sso]}>
      <ContextWrapper>
        <AuthConnectors />
      </ContextWrapper>
    </MemoryRouter>
  );
}
Failed.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getAuthConnectorsListUrl(), () =>
        HttpResponse.json(
          { message: 'something went wrong' },
          {
            status: 500,
          }
        )
      ),
    ],
  },
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
  return <ContextProvider ctx={ctx}>{children}</ContextProvider>;
}
