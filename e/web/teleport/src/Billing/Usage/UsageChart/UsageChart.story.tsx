import React from 'react';
import UsageChart from './UsageChart';

export default {
  title: 'TeleportE/Billing/Usage/Chart',
};

export const Loaded = () => {
  const arr = new Array(12);
  arr[1] = 16094;
  arr[2] = 0;
  arr[4] = 25000;
  arr[11] = 45000;
  return <UsageChart totalAmts={arr} />;
};

export const Empty = () => {
  return <UsageChart totalAmts={[]} />;
};
