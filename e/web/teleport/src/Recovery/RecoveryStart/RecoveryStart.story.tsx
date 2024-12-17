import { MemoryRouter } from 'react-router';
import { Attempt } from 'shared/hooks/useAttemptNext';

import { RecoveryStart } from './RecoveryStart';

export default {
  title: 'TeleportE/Recovery/RecoveryStart',
};

export const ForgotPassword = () => (
  <RecoveryStart {...props} recoveryType="password" />
);

export const ForgotMfa = () => (
  <MemoryRouter>
    <RecoveryStart {...props} recoveryType="device" />
  </MemoryRouter>
);

ForgotMfa.storyName = 'Forgot MFA';

export const Success = () => (
  <MemoryRouter>
    <RecoveryStart
      {...props}
      recoveryType="device"
      attempt={{ status: 'success' }}
    />
  </MemoryRouter>
);

const props = {
  attempt: { status: '' } as Attempt,
  submit: () => null,
};
