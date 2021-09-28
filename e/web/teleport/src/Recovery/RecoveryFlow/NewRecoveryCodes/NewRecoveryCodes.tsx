import React from 'react';
import { Box, Indicator } from 'design';
import { Danger } from 'design/Alert';
import RecoveryCodes from 'e-teleport/components/RecoveryCodes/RecoveryCodes';
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
      <Danger style={{ width: '504px', margin: 'auto' }}>
        {attempt.statusText}
      </Danger>
    );
  }

  return (
    <RecoveryCodes
      recoveryCodes={recoveryCodes}
      redirect={redirect}
      isNewCodes={true}
      continueText="Return to login"
    />
  );
}
