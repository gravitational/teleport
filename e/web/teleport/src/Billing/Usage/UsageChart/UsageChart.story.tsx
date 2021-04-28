import React from 'react';
import UsageChart from './UsageChart';

export default {
  title: 'TeleportE/Billing/Usage/Chart',
};

export const Loaded = () => {
  const arr = new Array(12);
  arr[1] = 160.94;
  arr[2] = 0;
  arr[4] = 250.0;
  arr[11] = 450.0;
  return <UsageChart totalAmts={arr} />;
};

export const Empty = () => {
  return <UsageChart totalAmts={[]} />;
};

export const ZeroValues = () => {
  const arr = new Array(12);
  arr[5] = 0;
  arr[11] = 0;
  return <UsageChart totalAmts={arr} />;
};
