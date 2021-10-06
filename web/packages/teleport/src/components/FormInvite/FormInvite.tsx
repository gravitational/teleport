/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { useState, useMemo } from 'react';
import { Text, Card, ButtonPrimary, Flex, Box, Link } from 'design';
import * as Alerts from 'design/Alert';
import { Auth2faType } from 'shared/services';
import { Attempt } from 'shared/hooks/useAttemptNext';
import FieldSelect from 'shared/components/FieldSelect';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import {
  requiredToken,
  requiredPassword,
  requiredConfirmedPassword,
} from 'shared/components/Validation/rules';
import { getMfaOptions, MfaOption } from 'teleport/services/mfa/utils';
import TwoFAData from './TwoFaInfo';

const U2F_ERROR_CODES_URL =
  'https://developers.yubico.com/U2F/Libraries/Client_error_codes.html';

export default function FormInvite(props: Props) {
  const {
    auth2faType,
    onSubmitWithU2f,
    onSubmit,
    attempt,
    clearSubmitAttempt,
    user,
    qr,
    title = '',
    submitBtnText = 'Submit',
  } = props;

  const mfaEnabled = auth2faType === 'on' || auth2faType === 'optional';
  const u2fEnabled = auth2faType === 'u2f';
  const otpEnabled = auth2faType === 'otp';
  const secondFactorEnabled = otpEnabled || u2fEnabled || mfaEnabled;

  const [password, setPassword] = useState('');
  const [passwordConfirmed, setPasswordConfirmed] = useState('');
  const [token, setToken] = useState('');

  const mfaOptions = useMemo<MfaOption[]>(() => getMfaOptions(auth2faType), []);

  const [mfaType, setMfaType] = useState(mfaOptions[0]);

  const boxWidth =
    (secondFactorEnabled && mfaType.value !== 'optional' ? 720 : 464) + 'px';

  function onBtnClick(
    e: React.MouseEvent<HTMLButtonElement>,
    validator: Validator
  ) {
    e.preventDefault();
    if (!validator.validate()) {
      return;
    }

    if (mfaType.value === 'u2f') {
      onSubmitWithU2f(password);
    } else {
      onSubmit(password, token);
    }
  }

  function onSetMfaOption(option: MfaOption, validator: Validator) {
    setToken('');
    clearSubmitAttempt();
    validator.reset();
    setMfaType(option);
  }

  return (
    <Validation>
      {({ validator }) => (
        <Card as="form" bg="primary.light" my={6} mx="auto" width={boxWidth}>
          <Flex>
            <Box flex="3" p="6">
              <Text typography="h2" mb={3} textAlign="center" color="light">
                {title}
              </Text>
              {attempt.status === 'failed' && (
                <ErrorMessage message={attempt.statusText} />
              )}
              <Text typography="h4" breakAll mb={3}>
                {user}
              </Text>
              <FieldInput
                rule={requiredPassword}
                autoFocus
                autoComplete="off"
                label="Password"
                value={password}
                onChange={e => setPassword(e.target.value)}
                type="password"
                placeholder="Password"
              />
              <FieldInput
                rule={requiredConfirmedPassword(password)}
                autoComplete="off"
                label="Confirm Password"
                value={passwordConfirmed}
                onChange={e => setPasswordConfirmed(e.target.value)}
                type="password"
                placeholder="Confirm Password"
              />
              {secondFactorEnabled && (
                <Flex alignItems="center">
                  <FieldSelect
                    maxWidth="50%"
                    width="100%"
                    data-testid="mfa-select"
                    label="Two-factor type"
                    value={mfaType}
                    options={mfaOptions}
                    onChange={opt =>
                      onSetMfaOption(opt as MfaOption, validator)
                    }
                    mr={3}
                    isDisabled={attempt.status === 'processing'}
                  />
                  {mfaType.value === 'otp' && (
                    <FieldInput
                      width="50%"
                      label="Authenticator code"
                      rule={requiredToken}
                      autoComplete="off"
                      value={token}
                      onChange={e => setToken(e.target.value)}
                      placeholder="123 456"
                    />
                  )}
                  {mfaType.value === 'u2f' &&
                    attempt.status === 'processing' && (
                      <Text typography="body2">
                        Insert your hardware key and press the button on the
                        key.
                      </Text>
                    )}
                </Flex>
              )}
              <ButtonPrimary
                width="100%"
                mt={3}
                disabled={attempt.status === 'processing'}
                size="large"
                onClick={e => onBtnClick(e, validator)}
              >
                {submitBtnText}
              </ButtonPrimary>
            </Box>
            {secondFactorEnabled && mfaType.value !== 'optional' && (
              <Box
                flex="1"
                bg="primary.main"
                p={6}
                borderTopRightRadius={3}
                borderBottomRightRadius={3}
              >
                <TwoFAData
                  auth2faType={mfaType.value}
                  qr={qr}
                  submitBtnText={submitBtnText}
                />
              </Box>
            )}
          </Flex>
        </Card>
      )}
    </Validation>
  );
}

export type Props = {
  title?: string;
  submitBtnText?: string;
  user: string;
  qr: string;
  auth2faType: Auth2faType;
  attempt: Attempt;
  clearSubmitAttempt: () => void;
  onSubmitWithU2f(password: string): void;
  onSubmit(password: string, optToken: string): void;
};

function ErrorMessage({ message = '' }) {
  // quick fix: check if error text has U2F substring
  const browserSupported = !message.includes('does not support U2F');
  const showU2fErrorLink = browserSupported && message.includes('U2F');

  return (
    <Alerts.Danger>
      <div>
        {message}
        {showU2fErrorLink && (
          <span>
            , click{' '}
            <Link target="_blank" href={U2F_ERROR_CODES_URL}>
              here
            </Link>{' '}
            to learn more about U2F error codes
          </span>
        )}
      </div>
    </Alerts.Danger>
  );
}
