import React from 'react';
import Component from './RecoveryCodes';

export default {
  title: 'TeleportE/RecoveryCodes',
};

export const FromInvite = () => <Component {...props} />;

export const FromReset = () => (
  <Component {...props} isNewCodes={true} continueText="Return to login" />
);

const props = {
  recoveryCodes: [
    'tele-testword-testword-testword-testword-testword-testword-testword',
    'tele-testword-testword-testword-testword-testword-testword-testword-testword',
    'tele-testword-testword-testword-testword-testword-testword-testword',
  ],
  redirect: () => null,
  isNewCodes: false,
};
