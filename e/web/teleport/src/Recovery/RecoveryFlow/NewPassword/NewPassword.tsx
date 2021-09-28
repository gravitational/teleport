import React, { useState } from 'react';
import { Card, ButtonPrimary, Text, Box } from 'design';
import { Danger } from 'design/Alert';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import {
  requiredPassword,
  requiredConfirmedPassword,
} from 'shared/components/Validation/rules';
import useNewPassword, { State, Props } from './useNewPassword';

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
    <Card as="form" bg="primary.light" mx="auto" width="512px">
      <Validation>
        {({ validator }) => (
          <>
            <Text typography="h3" pt={5} textAlign="center" color="light">
              Create a New Password
            </Text>
            <Text textAlign="center" color="text.secondary">
              Step 2 of 3
            </Text>
            <Box p={5}>
              {attempt.status === 'failed' && (
                <Danger width="100%">{attempt.statusText}</Danger>
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
              />
              <FieldInput
                rule={requiredConfirmedPassword(password)}
                label="Confirm New Password"
                placeholder="Confirm Password"
                value={passwordConfirmed}
                type="password"
                onChange={e => setPasswordConfirmed(e.target.value)}
                readonly={attempt.status === 'processing'}
              />
              <ButtonPrimary
                mt={3}
                size="large"
                width="100%"
                type="submit"
                onClick={e => onSubmit(e, validator)}
                disabled={attempt.status === 'processing'}
              >
                Continue
              </ButtonPrimary>
            </Box>
          </>
        )}
      </Validation>
    </Card>
  );
}
