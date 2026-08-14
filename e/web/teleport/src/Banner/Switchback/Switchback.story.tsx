import { Switchback } from './Switchback';

export default {
  title: 'TeleportE/Banner/Switchback',
};

export const Loaded = () => {
  return <Switchback {...props} />;
};

export const ManyRoles = () => {
  return (
    <Switchback
      {...props}
      assumedRoles={Array.from({ length: 10 }, (_, i) => `role${i + 1}`)}
    />
  );
};

export const Processing = () => {
  return <Switchback {...props} attempt={{ status: 'processing' }} />;
};

export const Failed = () => {
  return (
    <Switchback
      {...props}
      attempt={{ status: 'failed', statusText: 'server error' }}
    />
  );
};

const props = {
  assumedRoles: ['devA', 'devB', 'devC'],
  time: {
    hours: 2,
    minutes: 35,
    seconds: 0,
  },
  btnSetting: {
    text: 'Switch Back',
    func: () => null,
  },
  attempt: { status: '' as any },
  onErrorConfirm: () => null,
};
