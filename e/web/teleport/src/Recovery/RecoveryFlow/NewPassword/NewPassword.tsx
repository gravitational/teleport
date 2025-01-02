import React, { useState } from 'react';

import { Box, ButtonPrimary, Card } from 'design';
import { OutlineDanger } from 'design/Alert/Alert';
import { StepHeader } from 'design/StepSlider';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import {
  requiredConfirmedPassword,
  requiredPassword,
} from 'shared/components/Validation/rules';

import useNewPassword, { Props, State } from './useNewPassword';

export default function Container(props: Props) {
  const state = useNewPassword(props);
  return <NewPassword {...state} />;
}

export function NewPassword({ setNewPassword, attempt }: State) {
  const [password, setPassword] = useState('');
  const [passwordConfirmed, setPasswordConfirmed] = useState('');

  function onSubmit(
    e: React.MouseEvent<HTMLButtonElement>,
    validator: Validator
  ) {
    e.preventDefault();
    if (!validator.validate()) {
      return;
    }

    setNewPassword(password);
  }

  return (
    <Card as="form" mx="auto" width="512px" p={4}>
      <Validation>
        {({ validator }) => (
          <>
            <Box mb={4}>
              <StepHeader
                stepIndex={1}
                flowLength={3}
                title="Create a New Password"
              />
            </Box>
            {attempt.status === 'failed' && (
              <OutlineDanger width="100%">{attempt.statusText}</OutlineDanger>
            )}
            <FieldInput
              rule={requiredPassword}
              autoFocus
              label="New Password"
              placeholder="Password"
              value={password}
              type="password"
              onChange={e => setPassword(e.target.value)}
              readonly={attempt.status === 'processing'}
              mb={3}
            />
            <FieldInput
              rule={requiredConfirmedPassword(password)}
              label="Confirm New Password"
              placeholder="Confirm Password"
              value={passwordConfirmed}
              type="password"
              onChange={e => setPasswordConfirmed(e.target.value)}
              readonly={attempt.status === 'processing'}
              mb={3}
            />
            <ButtonPrimary
              size="large"
              width="100%"
              type="submit"
              onClick={e => onSubmit(e, validator)}
              disabled={attempt.status === 'processing'}
            >
              Continue
            </ButtonPrimary>
          </>
        )}
      </Validation>
    </Card>
  );
}
