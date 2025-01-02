import { RecoveryToken } from 'e-teleport/services/recovery/types';

import { State } from './useVerifyUser';
import { VerifyUser } from './VerifyUser';

export default {
  title: 'TeleportE/Recovery/Flow/Step 1/Verify Multi',
};

export const Loaded = () => <VerifyUser {...props} />;

export const Processing = () => (
  <VerifyUser {...props} attempt={{ status: 'processing' }} />
);

export const Failed = () => (
  <VerifyUser
    {...props}
    attempt={{ status: 'failed', statusText: 'error message' }}
  />
);

const props: State = {
  token: {
    username: 'joe@example.com',
    isRecoverPassword: true,
  } as RecoveryToken,
  auth2faType: 'on',
  preferredMfaType: 'webauthn',
  attempt: { status: '' },
  submitPasswordCreds: () => null,
  submitTotpCreds: () => null,
  submitWebauthnCreds: () => null,
};
