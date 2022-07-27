import React from 'react';

import { ScheduleUpgrades, Props } from './ScheduleUpgrades';

export default {
  title: 'Teleport/Support/ScheduleUpgrades',
};

export const Loaded = () => {
  return <ScheduleUpgrades {...props} />;
};

export const Processing = () => {
  return (
    <ScheduleUpgrades
      {...props}
      attempt={{
        status: 'processing',
        statusText: '',
      }}
    />
  );
};

export const Failed = () => {
  return (
    <ScheduleUpgrades
      {...props}
      attempt={{
        status: 'failed',
        statusText: 'there was an error processing the request',
      }}
    />
  );
};

const props: Props = {
  onCancel: () => null,
  onSave: () => null,
  attempt: {
    status: '',
    statusText: '',
  },
  onSelectedWindowChange: () => null,
  selectedWindow: '08:00:00',
};
