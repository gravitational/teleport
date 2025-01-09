/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React, { useEffect, useState } from 'react';

import {
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Indicator,
  Text,
} from 'design';
import { Alert, Danger } from 'design/Alert';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import FieldInput from 'shared/components/FieldInput';
import FieldSelect from 'shared/components/FieldSelect';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredToken } from 'shared/components/Validation/rules';

import { MfaOption } from 'teleport/services/mfa';

import useReAuthenticate, {
  ReauthProps,
  ReauthState,
} from './useReAuthenticate';

export type Props = ReauthProps & {
  onClose: () => void;
};

export default function Container(props: Props) {
  const state = useReAuthenticate(props);
  return <ReAuthenticate onClose={props.onClose} reauthState={state} />;
}

export type State = {
  reauthState: ReauthState;
  onClose: () => void;
};

export function ReAuthenticate({
  onClose,
  reauthState: {
    initAttempt,
    mfaOptions,
    submitWithMfa,
    submitAttempt,
    clearSubmitAttempt,
  },
}: State) {
  const [otpCode, setOtpToken] = useState('');
  const [mfaOption, setMfaOption] = useState<MfaOption>();

  useEffect(() => {
    if (mfaOptions?.length) setMfaOption(mfaOptions[0]);
  }, [mfaOptions]);

  // Handle potential error states first.
  switch (initAttempt.status) {
    case 'processing':
      return (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      );
    case 'error':
      return <Alert children={initAttempt.statusText} />;
    case 'success':
      break;
    default:
      return null;
  }

  function onReauthenticate(
    e: React.MouseEvent<HTMLButtonElement>,
    validator: Validator
  ) {
    e.preventDefault();
    if (!validator.validate()) return;
    submitWithMfa(mfaOption.value, 'mfa', otpCode);
  }

  return (
    <Validation>
      {({ validator }) => (
        <Dialog
          dialogCss={() => ({
            width: '416px',
          })}
          disableEscapeKeyDown={false}
          onClose={onClose}
          open={true}
        >
          <form>
            <DialogHeader style={{ flexDirection: 'column' }}>
              <DialogTitle>Verify your identity</DialogTitle>
              <Text textAlign="center" color="text.slightlyMuted">
                You must verify your identity with one of your existing
                two-factor devices before performing this action.
              </Text>
            </DialogHeader>
            {submitAttempt.status === 'error' && (
              <Danger mt={2} width="100%">
                {submitAttempt.statusText}
              </Danger>
            )}
            <DialogContent>
              <Flex mt={2} alignItems="flex-start">
                <FieldSelect
                  width="60%"
                  label="Two-factor Type"
                  value={mfaOption}
                  options={mfaOptions}
                  onChange={(o: MfaOption) => {
                    setMfaOption(o);
                    clearSubmitAttempt();
                  }}
                  data-testid="mfa-select"
                  mr={3}
                  mb={0}
                  isDisabled={submitAttempt.status === 'processing'}
                  elevated={true}
                />
                <Box width="40%">
                  {mfaOption?.value === 'totp' && (
                    <FieldInput
                      label="Authenticator Code"
                      rule={requiredToken}
                      inputMode="numeric"
                      autoComplete="one-time-code"
                      value={otpCode}
                      onChange={e => setOtpToken(e.target.value)}
                      placeholder="123 456"
                      readonly={submitAttempt.status === 'processing'}
                      mb={0}
                    />
                  )}
                </Box>
              </Flex>
            </DialogContent>
            <DialogFooter>
              <ButtonPrimary
                onClick={e => onReauthenticate(e, validator)}
                disabled={submitAttempt.status === 'processing'}
                mr={3}
                mt={3}
                type="submit"
                autoFocus
              >
                Continue
              </ButtonPrimary>
              <ButtonSecondary onClick={onClose}>Cancel</ButtonSecondary>
            </DialogFooter>
          </form>
        </Dialog>
      )}
    </Validation>
  );
}
