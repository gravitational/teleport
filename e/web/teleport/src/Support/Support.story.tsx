import React from 'react';
import { MemoryRouter } from 'react-router';
import { ContextProvider } from 'teleport';
import { Support } from 'teleport/Support';
import { ContentMinWidth } from 'teleport/Main/Main';
import cfg from 'teleport/config';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import { SupportE } from './Support';

import type { Props } from './Support';

export default {
  title: 'TeleportE/Support',
};

export const WithCloudSection = () => {
  const ctx = createTeleportContextE();
  ctx.hasExternalAuditStorage = false;
  cfg.isCloud = true;

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <ContentMinWidth>
          <Support>
            <SupportE {...props} isCloud={true} />
          </Support>
        </ContentMinWidth>
      </ContextProvider>
    </MemoryRouter>
  );
};

const props: Props = {
  fetchWindowAttempt: {
    status: 'success',
    statusText: '',
  },
  updateWindowAttempt: {
    status: '',
    statusText: '',
  },
  closeScheduleUpgrade: () => null,
  isCloud: false,
  onUpdate: () => null,
  scheduleUpgradesVisible: false,
  selectedUpgradeWindowStart: 8,
  setSelectedUpgradeWindowStart: () => null,
  showScheduleUpgrade: () => null,
};
