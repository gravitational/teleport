import React from 'react';
import { Box, Indicator } from 'design';
import RecoveryCodes from 'teleport/components/RecoveryCodes';

import { OutlineDanger } from 'design/Alert/Alert';

import useNewRecoveryCodes, { State, Props } from './useNewRecoveryCodes';

export default function Container(props: Props) {
  const state = useNewRecoveryCodes(props);
  return <NewRecoveryCodes {...state} />;
}

export function NewRecoveryCodes({ attempt, recoveryCodes, redirect }: State) {
  if (attempt.status === 'processing') {
    return (
      <Box textAlign="center">
        <Indicator />
      </Box>
    );
  }

  if (attempt.status === 'failed') {
    return (
      <OutlineDanger style={{ width: '504px', margin: 'auto' }}>
        {attempt.statusText}
      </OutlineDanger>
    );
  }

  return (
    <RecoveryCodes
      recoveryCodes={recoveryCodes}
      onContinue={redirect}
      isNewCodes={true}
      continueText="Return to login"
    />
  );
}
