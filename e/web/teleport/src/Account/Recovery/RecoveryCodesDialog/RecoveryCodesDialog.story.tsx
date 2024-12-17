import { RecoveryCodesDialog } from './RecoveryCodesDialog';
import { State } from './useRecoveryCodesDialog';

export default {
  title: 'TeleportE/Account/Recovery/Recovery Codes Dialog',
};

export const Loaded = () => <RecoveryCodesDialog {...props} />;

export const LoadedFirstTime = () => (
  <RecoveryCodesDialog {...props} isNewCodes={false} />
);

export const Processing = () => (
  <RecoveryCodesDialog {...props} attempt={{ status: 'processing' }} />
);

export const Failed = () => (
  <RecoveryCodesDialog
    {...props}
    attempt={{
      status: 'failed',
      statusText: 'failed to generate recovery codes',
    }}
  />
);

const props: State = {
  attempt: { status: 'success' },
  close: () => null,
  closeWithDateRefresh: () => null,
  generateCodes: () => null,
  isNewCodes: true,
  recoveryCodes: {
    codes: [
      'tele-testword-testword-testword-testword-testword-testword-testword',
      'tele-testword-testword-testword-testword-testword-testword-testword',
      'tele-testword-testword-testword-testword-testword-testword-testword',
    ],
    createdDate: new Date('2019-08-30T11:00:00.00Z'),
  },
};
