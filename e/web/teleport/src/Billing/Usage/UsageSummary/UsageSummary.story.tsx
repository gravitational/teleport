import React from 'react';

import { MonthlyItem } from '../useUsage';

import UsageSummary from './UsageSummary';

export default {
  title: 'TeleportE/Billing/Usage/Summary',
};

export const Loaded = () => {
  return <UsageSummary items={items} />;
};

export const Empty = () => {
  return <UsageSummary items={[]} />;
};

const items: MonthlyItem[] = [
  {
    resource: 'Applications',
    jan: 12,
    feb: 25,
  },
  {
    resource: 'Databases',
    jan: 1,
    feb: 4,
  },
  {
    resource: 'Kubernetes',
    jan: 0,
    feb: 5,
  },
  {
    resource: 'Servers',
    feb: 10000,
  },
  {
    resource: 'Active Users',
    feb: 9999999,
  },
];
