import { RecoveryToken } from 'e-teleport/services/recovery/types';

import { State } from './useVerifyUser';
import { VerifyUser } from './VerifyUser';

export default {
  title: 'TeleportE/Recovery/Flow/Step 1/Verify Multi',
};

export const Loaded = () => <VerifyUser {...props} />;

export const Processing = () => (
  <VerifyUser {...props} submitAttempt={{ status: 'processing' }} />
);

export const Failed = () => (
  <VerifyUser
    {...props}
    submitAttempt={{ status: 'failed', statusText: 'error message' }}
  />
);

const props: State = {
  token: {
    username: 'joe@example.com',
    isRecoverPassword: true,
  } as RecoveryToken,
  submitAttempt: { status: '' },
  submitWithPassword: () => null,
  submitWithMfa: () => null,
};
