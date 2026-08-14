import { http, HttpResponse } from 'msw';
import { useState } from 'react';
import { MemoryRouter } from 'react-router';

import { Flex } from 'design';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport';
import { InfoGuideSidePanel } from 'teleport/components/SlidingSidePanel/InfoGuideSidePanel';
import {
  mockClusterMaintenance,
  mockManagedUpdatesNotConfiguredCloud,
  mockManagedUpdatesTimeBasedCloud,
  mockManagedUpdatesWithOrphaned,
} from 'teleport/ManagedUpdates/fixtures';

import ManagedUpdates from './ManagedUpdates';

export default {
  title: 'TeleportE/ManagedUpdates',
};

function createStoryContext() {
  const ctx = createTeleportContextE();
  cfg.oss.isCloud = true;
  ctx.storeUser.state.cluster.authVersion = '18.2.1';
  return ctx;
}

const StoryContainer = ({ children }: { children: React.ReactNode }) => {
  const [ctx] = useState(() => createStoryContext());
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <InfoGuidePanelProvider>
          <Flex
            height="100vh"
            flexDirection="column"
            css={`
              // Override the top offset for slideout panel since there's no navbar in storybook.
              & + div {
                top: 0;
              }
            `}
          >
            {children}
          </Flex>
          <InfoGuideSidePanel />
        </InfoGuidePanelProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};

export function LoadedCloud() {
  return (
    <StoryContainer>
      <ManagedUpdates />
    </StoryContainer>
  );
}
LoadedCloud.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.oss.getManagedUpdatesUrl(), () => {
      return HttpResponse.json(mockManagedUpdatesTimeBasedCloud);
    }),
    http.post(cfg.getWindowUpgradeStartUrl('aws'), windowPostHandler),
    http.get(cfg.api.environmentProfileUrl, () => {
      return HttpResponse.json({ environmentProfile: 'production' });
    }),
    http.post(cfg.api.environmentProfileUrl, environmentPostHandler)
  );
};

export function CloudWithOrphanedAgents() {
  return (
    <StoryContainer>
      <ManagedUpdates />
    </StoryContainer>
  );
}
CloudWithOrphanedAgents.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.oss.getManagedUpdatesUrl(), () => {
      return HttpResponse.json({
        ...mockManagedUpdatesWithOrphaned,
        clusterMaintenance: mockClusterMaintenance,
      });
    }),
    http.post(cfg.getWindowUpgradeStartUrl('aws'), windowPostHandler),
    http.get(cfg.api.environmentProfileUrl, () => {
      return HttpResponse.json({ environmentProfile: 'production' });
    }),
    http.post(cfg.api.environmentProfileUrl, environmentPostHandler)
  );
};

export function NotConfiguredCloud() {
  return (
    <StoryContainer>
      <ManagedUpdates />
    </StoryContainer>
  );
}
NotConfiguredCloud.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.oss.getManagedUpdatesUrl(), () => {
      return HttpResponse.json(mockManagedUpdatesNotConfiguredCloud);
    }),
    http.post(cfg.getWindowUpgradeStartUrl('aws'), windowPostHandler),
    http.get(cfg.api.environmentProfileUrl, () => {
      return HttpResponse.json({ environmentProfile: 'production' });
    }),
    http.post(cfg.api.environmentProfileUrl, environmentPostHandler)
  );
};

async function environmentPostHandler(req) {
  const { environmentProfile } = (await req.request.json()) as any;

  return HttpResponse.json({
    environmentProfile: environmentProfile,
  });
}

async function windowPostHandler(req) {
  const { upgradeWindowStartHour } = (await req.request.json()) as any;

  if (upgradeWindowStartHour === 16) {
    return HttpResponse.json(
      { error: 'Internal Server Error' },
      { status: 500, statusText: 'Internal Server Error' }
    );
  }

  return HttpResponse.json({
    upgradeWindowStartHour: upgradeWindowStartHour,
  });
}
