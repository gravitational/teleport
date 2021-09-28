import React from 'react';
import { Auth2faType } from 'shared/services';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { RecoveryToken } from 'e-teleport/services/recovery/types';
import { VerifyUser } from './VerifyUser';

export default {
  title: 'TeleportE/Recovery/Flow/Step 1/Verify TOTP',
};

export const Loaded = () => <VerifyUser {...props} />;

export const Processing = () => (
  <VerifyUser {...props} attempt={{ status: 'processing' }} />
);

export const Failed = () => (
  <VerifyUser
    {...props}
    attempt={{ status: 'failed', statusText: 'wrong totp token' }}
  />
);

const props = {
  token: {
    username: 'joe@example.com',
    isRecoverPassword: true,
  } as RecoveryToken,
  auth2faType: 'otp' as Auth2faType,
  attempt: { status: '' } as Attempt,
  submitPasswordCreds: () => null,
  submitTotpCreds: () => null,
  submitU2fCreds: () => null,
};
