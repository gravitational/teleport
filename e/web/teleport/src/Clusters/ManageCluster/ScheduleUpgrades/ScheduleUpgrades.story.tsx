import { Props, ScheduleUpgrades } from './ScheduleUpgrades';

export default {
  title: 'Teleport/Clusters/ManageClusters/ScheduleUpgrades',
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
  selectedWindow: 8,
};
