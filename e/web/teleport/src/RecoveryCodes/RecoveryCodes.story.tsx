import { RecoveryCodes } from './RecoveryCodes';

export default {
  title: 'TeleportE/RecoveryCodes',
};

export const FromInvite = () => <RecoveryCodes {...props} />;

export const FromReset = () => (
  <RecoveryCodes {...props} isNewCodes={true} continueText="Return to Login" />
);

const props = {
  recoveryCodes: {
    codes: [
      'tele-testword-testword-testword-testword-testword-testword-testword-testword',
      'tele-testword-testword-testword-testword-testword-testword-testword-testword',
      'tele-testword-testword-testword-testword-testword-testword-testword-testword',
    ],
    createdDate: new Date('2019-08-30T11:00:00.00Z'),
  },
  onContinue: () => null,
  isNewCodes: false,
};
