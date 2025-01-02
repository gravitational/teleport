import { RecoveryToken } from 'e-teleport/services/recovery/types';

import { State } from './useVerifyUser';
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

const props: State = {
  token: {
    username: 'joe@example.com',
    isRecoverPassword: false,
  } as RecoveryToken,
  auth2faType: 'on',
  preferredMfaType: '',
  attempt: { status: '' },
  submitPasswordCreds: () => null,
  submitTotpCreds: () => null,
  submitWebauthnCreds: () => null,
};
