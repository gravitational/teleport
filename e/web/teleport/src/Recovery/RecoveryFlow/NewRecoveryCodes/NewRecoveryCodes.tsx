import { Box, Indicator } from 'design';
import { OutlineDanger } from 'design/Alert/Alert';

import { RecoveryCodes } from 'e-teleport/RecoveryCodes';

import useNewRecoveryCodes, { Props, State } from './useNewRecoveryCodes';

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
      continueText="Return to Login"
    />
  );
}
