import React from 'react';
import { Recovery } from './Recovery';
import { State } from './useRecovery';

export default {
  title: 'TeleportE/Account/Recovery',
};

export const Loaded = () => <Recovery {...props} />;

export const LoadedFirstTime = () => (
  <Recovery {...props} userHasCodes={false} />
);

export const LoadedNoEmailAddress = () => (
  <Recovery {...props} userHasCodes={false} isRecoveryEnabled={false} />
);

export const Processing = () => (
  <Recovery {...props} attempt={{ status: 'processing' }} />
);

export const Failed = () => (
  <Recovery
    {...props}
    attempt={{
      status: 'failed',
      statusText: 'failed to fetch date of codes generation',
    }}
  />
);

const props: State = {
  attempt: { status: 'success' },
  token: '',
  createdDate: new Date('2020-10-09T17:40:04.134157474Z'),
  userHasCodes: true,
  setToken: () => null,
  isReAuthenticateVisible: false,
  isCodesVisible: false,
  showReAuthenticate: () => null,
  hideReAuthenticate: () => null,
  hideCodes: () => null,
  fetchCreatedDate: () => null,
  isRecoveryEnabled: true,
};
