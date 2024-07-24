import React from 'react';
import { MemoryRouter } from 'react-router';
import { ContextProvider } from 'teleport';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import { SupportE } from './Support';

import type { Props } from './Support';

export default {
  title: 'TeleportE/Support',
};

export const CloudSection = () => (
  <MemoryRouter>
    <ContextProvider ctx={createTeleportContextE()}>
      <SupportE {...props} isCloud={true}></SupportE>
    </ContextProvider>
  </MemoryRouter>
);

const props: Props = {
  attempt: {
    status: 'success',
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
