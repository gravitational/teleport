import React from 'react';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { Auth2faType } from 'shared/services';
import { RecoveryToken } from 'e-teleport/services/recovery/types';
import { VerifyUser } from './VerifyUser';

export default {
  title: 'TeleportE/Recovery/Flow/Step 1/Verify Password',
};

export const Loaded = () => <VerifyUser {...props} />;

export const Processing = () => (
  <VerifyUser {...props} attempt={{ status: 'processing' }} />
);

export const Failed = () => (
  <VerifyUser
    {...props}
    attempt={{ status: 'failed', statusText: 'wrong password' }}
  />
);

const props = {
  token: {
    username: 'joe@example.com',
    isRecoverPassword: false,
  } as RecoveryToken,
  auth2faType: 'on' as Auth2faType,
  attempt: { status: '' } as Attempt,
  submitPasswordCreds: () => null,
  submitTotpCreds: () => null,
  submitU2fCreds: () => null,
};
