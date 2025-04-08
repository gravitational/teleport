import { LocationDescriptor } from 'history';
import { PropsWithChildren } from 'react';
import { MemoryRouter } from 'react-router';

import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { getEnterpriseFeatures } from 'e-teleport/features';
import TeleportContextE from 'e-teleport/teleportContextE';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { ContextProvider } from 'teleport/index';

import { createTeleportContextE } from './contexts';

export const TeleportProviderBasicE: React.FC<
  PropsWithChildren<{
    initialEntries?: LocationDescriptor<unknown>[];
    teleportCtx?: TeleportContextE;
  }>
> = ({ children, initialEntries, teleportCtx }) => {
  const ctx = teleportCtx || createTeleportContextE();

  return (
    <MemoryRouter initialEntries={initialEntries}>
      <InfoGuidePanelProvider>
        <ContextProvider ctx={ctx}>
          <FeaturesContextProvider value={getEnterpriseFeatures()}>
            {children}
          </FeaturesContextProvider>
        </ContextProvider>
      </InfoGuidePanelProvider>
    </MemoryRouter>
  );
};
