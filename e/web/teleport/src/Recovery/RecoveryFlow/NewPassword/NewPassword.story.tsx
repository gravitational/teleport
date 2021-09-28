import React from 'react';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { NewPassword } from './NewPassword';

export default {
  title: 'TeleportE/Recovery/Flow/Step 2/New Password',
};

export const Loaded = () => <NewPassword {...props} />;

export const Processing = () => (
  <NewPassword {...props} attempt={{ status: 'processing' }} />
);

export const Failed = () => (
  <NewPassword
    {...props}
    attempt={{
      status: 'failed',
      statusText: 'failed to reset password',
    }}
  />
);

const props = {
  setNewPassword: () => null,
  attempt: { status: '' } as Attempt,
};
