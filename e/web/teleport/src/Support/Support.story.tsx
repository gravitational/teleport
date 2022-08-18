import React from 'react';

import { SupportE } from './Support';

import type { Props } from './Support';

export default {
  title: 'TeleportE/Support',
};

export const CloudSection = () => (
  <SupportE {...props} isCloud={true}></SupportE>
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
