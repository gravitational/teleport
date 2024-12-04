import React from 'react';
import { MemoryRouter } from 'react-router';
import { ContextProvider } from 'teleport';
import { Support } from 'teleport/Support';
import { ContentMinWidth } from 'teleport/Main/Main';
import cfg from 'teleport/config';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import { SupportE } from './Support';

export default {
  title: 'TeleportE/Support',
};

export const WithoutExternalAuditStorages = () => {
  const ctx = createTeleportContextE();
  ctx.hasExternalAuditStorage = false;
  cfg.isCloud = false;

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <ContentMinWidth>
          <Support>
            <SupportE />
          </Support>
        </ContentMinWidth>
      </ContextProvider>
    </MemoryRouter>
  );
};

export const WithExternalAuditStorageCTA = () => {
  const ctx = createTeleportContextE();
  ctx.hasExternalAuditStorage = false;
  cfg.isCloud = true;

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <ContentMinWidth>
          <Support>
            <SupportE />
          </Support>
        </ContentMinWidth>
      </ContextProvider>
    </MemoryRouter>
  );
};
