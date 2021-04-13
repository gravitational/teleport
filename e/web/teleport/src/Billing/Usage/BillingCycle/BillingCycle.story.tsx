import React from 'react';
import BillingCycle from './BillingCycle';
import { cycles } from 'e-teleport/Billing/fixtures';

export default {
  title: 'TeleportE/Billing/Usage/BillingCycle',
};

export const Loaded = () => {
  return <BillingCycle cycles={cycles} balance="$123,456.00" />;
};

export const Empty = () => {
  return <BillingCycle cycles={[]} balance="$0.00" />;
};
