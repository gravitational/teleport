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

import React, { useState } from 'react';
import { Flex, Box, Text, ButtonPrimary, ButtonSecondary } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import { Danger } from 'design/Alert';
import Validation from 'shared/components/Validation';
import { requiredToken } from 'shared/components/Validation/rules';
import FieldInput from 'shared/components/FieldInput';
import FieldSelect from 'shared/components/FieldSelect';
import createMfaOptions, { MfaOption } from 'shared/utils/createMfaOptions';

import useReAuthenticate, { State, Props } from './useReAuthenticate';

export default function Container(props: Props) {
  const state = useReAuthenticate(props);
  return <ReAuthenticate {...state} />;
}

export function ReAuthenticate({
  attempt,
  clearAttempt,
  submitWithTotp,
  submitWithWebauthn,
  onClose,
  auth2faType,
  preferredMfaType,
  actionText,
}: State) {
  const [otpToken, setOtpToken] = useState('');
  const mfaOptions = createMfaOptions({
    auth2faType: auth2faType,
    preferredType: preferredMfaType,
    required: true,
  });
  const [mfaOption, setMfaOption] = useState<MfaOption>(mfaOptions[0]);

  function onSubmit(e: React.MouseEvent<HTMLButtonElement>) {
    e.preventDefault();

    if (mfaOption?.value === 'webauthn') {
      submitWithWebauthn();
    }
    if (mfaOption?.value === 'otp') {
      submitWithTotp(otpToken);
    }
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
                two-factor devices before {actionText}.
              </Text>
            </DialogHeader>
            {attempt.status === 'failed' && (
              <Danger mt={2} width="100%">
                {attempt.statusText}
              </Danger>
            )}
            <DialogContent>
              <Flex mt={2} alignItems="flex-end">
                <FieldSelect
                  width="60%"
                  label="Two-factor Type"
                  value={mfaOption}
                  options={mfaOptions}
                  onChange={(o: MfaOption) => {
                    setMfaOption(o);
                    clearAttempt();
                  }}
                  data-testid="mfa-select"
                  mr={3}
                  mb={0}
                  isDisabled={attempt.status === 'processing'}
                  elevated={true}
                />
                <Box width="40%">
                  {mfaOption.value === 'otp' && (
                    <FieldInput
                      label="Authenticator Code"
                      rule={requiredToken}
                      inputMode="numeric"
                      autoComplete="one-time-code"
                      value={otpToken}
                      onChange={e => setOtpToken(e.target.value)}
                      placeholder="123 456"
                      readonly={attempt.status === 'processing'}
                      mb={0}
                    />
                  )}
                </Box>
              </Flex>
            </DialogContent>
            <DialogFooter>
              <ButtonPrimary
                onClick={e => validator.validate() && onSubmit(e)}
                disabled={attempt.status === 'processing'}
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
