import React, { useState, useMemo } from 'react';
import { Card, Text, ButtonPrimary, CardSuccess, Box, H2 } from 'design';
import { Danger } from 'design/Alert';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import RecoveryService from 'e-teleport/services/recovery/recovery';

import useRecoveryStart, { State, RecoveryType } from './useRecoveryStart';

export default function Container({ recoveryType }: Props) {
  const recoveryService = useMemo(() => new RecoveryService(), []);
  const state = useRecoveryStart({
    recoveryService,
    recoveryType,
  });
  return <RecoveryStart {...state} />;
}

export function RecoveryStart({ submit, attempt, recoveryType }: State) {
  const [username, setUsername] = useState('');
  const [recoveryCode, setRecoveryCode] = useState('');

  function onBtnClick(
    e: React.MouseEvent<HTMLButtonElement>,
    validator: Validator
  ) {
    e.preventDefault();
    if (!validator.validate()) {
      return;
    }

    const isRecoverPassword = recoveryType === 'password';

    submit({
      username,
      recoveryCode,
      isRecoverPassword,
    });
  }

  if (attempt.status === 'success') {
    return (
      <CardSuccess title="Recovery Email Sent">
        Check your inbox for an email with the link to recover your account.
      </CardSuccess>
    );
  }

  const title =
    recoveryType === 'password'
      ? 'Password Recovery'
      : 'Multi-Factor Device Recovery';

  return (
    <Card as="form" mx="auto" width="650px">
      <Validation>
        {({ validator }) => (
          <>
            <H2 pt={4} textAlign="center">
              {title}
            </H2>
            <Box p={4}>
              {attempt.status === 'failed' && (
                <Danger width="100%">{attempt.statusText}</Danger>
              )}
              <Text color="text.slightlyMuted" mb={3}>
                Please enter the username for your account and one of your
                recovery codes. We'll send you an email with a link to the
                recovery process for your account.
              </Text>
              <FieldInput
                rule={requiredField('Username is required')}
                autoFocus
                label="Username"
                value={username}
                onChange={e => setUsername(e.target.value)}
                type="text"
                width="100%"
                mb={3}
              />
              <FieldInput
                rule={requiredField('Recovery code is required')}
                label="Recovery Code (Use 1 Recovery Code)"
                value={recoveryCode}
                onChange={e => setRecoveryCode(e.target.value)}
                type="text"
                width="100%"
                mb={3}
              />
              <ButtonPrimary
                size="large"
                type="submit"
                onClick={e => onBtnClick(e, validator)}
                width="100%"
                disabled={attempt.status === 'processing'}
              >
                Send Recovery Email
              </ButtonPrimary>
            </Box>
          </>
        )}
      </Validation>
    </Card>
  );
}

type Props = {
  recoveryType: RecoveryType;
};
