import React from 'react';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { NewRecoveryCodes } from './NewRecoveryCodes';

export default {
  title: 'TeleportE/Recovery/Flow/Step 4 (or 3)/New Recovery Codes',
};

export const Loaded = () => <NewRecoveryCodes {...props} />;

export const Loading = () => (
  <NewRecoveryCodes {...props} attempt={{ status: 'processing' }} />
);

export const Failed = () => (
  <NewRecoveryCodes
    {...props}
    attempt={{
      status: 'failed',
      statusText: 'failed to get new recovery codes',
    }}
  />
);

const props = {
  attempt: { status: 'success' } as Attempt,
  recoveryCodes: {
    codes: [
      'tele-testword-testword-testword-testword-testword-testword-testword',
      'tele-testword-testword-testword-testword-testword-testword-testword-testword',
      'tele-testword-testword-testword-testword-testword-testword-testword',
    ],
    createdDate: new Date('2019-08-30T11:00:00.00Z'),
  },
  redirect: () => null,
};
