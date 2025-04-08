import { MemoryRouter } from 'react-router';

import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { ContentMinWidth } from 'teleport/Main/Main';
import { Support } from 'teleport/Support';

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
        <InfoGuidePanelProvider>
          <ContentMinWidth>
            <Support>
              <SupportE />
            </Support>
          </ContentMinWidth>
        </InfoGuidePanelProvider>
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
        <InfoGuidePanelProvider>
          <ContentMinWidth>
            <Support>
              <SupportE />
            </Support>
          </ContentMinWidth>
        </InfoGuidePanelProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};
