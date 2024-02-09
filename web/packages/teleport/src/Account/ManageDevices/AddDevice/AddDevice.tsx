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

import React, { useState, useMemo } from 'react';
import {
  Text,
  Flex,
  Image,
  ButtonPrimary,
  ButtonSecondary,
  Link,
  Indicator,
} from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import { Danger } from 'design/Alert';
import FieldInput from 'shared/components/FieldInput';
import Validation from 'shared/components/Validation';
import {
  requiredToken,
  requiredField,
} from 'shared/components/Validation/rules';
import FieldSelect from 'shared/components/FieldSelect';

import createMfaOptions, { MfaOption } from 'shared/utils/createMfaOptions';

import secKeyGraphic from 'design/assets/images/sec-key-graphic.svg';

import { DeviceUsage } from 'teleport/services/mfa';
import useTeleport from 'teleport/useTeleport';

import useAddDevice, { State, Props } from './useAddDevice';

const deviceUsageOpts: DeviceusageOpt[] = [
  {
    value: 'mfa',
    label: 'no',
  },
  {
    value: 'passwordless',
    label: 'yes',
  },
];

export default function Container(props: Props) {
  const ctx = useTeleport();
  const state = useAddDevice(ctx, props);
  return (
    <AddDevice {...state} restrictDeviceUsage={props.restrictDeviceUsage} />
  );
}

export function AddDevice({
  addDeviceAttempt,
  fetchQrCodeAttempt,
  addTotpDevice,
  addWebauthnDevice,
  clearAttempt,
  onClose,
  qrCode,
  auth2faType,
  isPasswordlessEnabled,
  restrictDeviceUsage,
}: State) {
  const [otpToken, setOtpToken] = useState('');
  const [deviceName, setDeviceName] = useState('');

  const mfaOptions = useMemo(
    () => createMfaOptions({ auth2faType: auth2faType, required: true }),
    []
  );

  const [mfaOption, setMfaOption] = useState(mfaOptions[0]);
  const [usageOption, setUsageOption] = useState(deviceUsageOpts[0]);

  const resolvedDeviceUsage = restrictDeviceUsage ?? usageOption.value;

  function onSetMfaOption(option: MfaOption) {
    setOtpToken('');
    clearAttempt();
    setMfaOption(option);
  }

  function onSubmit(e: React.MouseEvent<HTMLButtonElement>) {
    e.preventDefault();

    if (mfaOption.value === 'webauthn') {
      addWebauthnDevice(deviceName, resolvedDeviceUsage);
    }
    if (mfaOption.value === 'otp') {
      addTotpDevice(otpToken, deviceName);
    }
  }

  let hardwareInstructions = 'Enter a name for your hardware key.';
  if (addDeviceAttempt.status === 'processing') {
    hardwareInstructions = 'Follow the prompts from your browser.';
  }

  const dialogTitle =
    restrictDeviceUsage === 'passwordless'
      ? 'Add New Passkey'
      : 'Add New Two-Factor Device';

  return (
    <Validation>
      {({ validator }) => (
        <Dialog
          dialogCss={() => ({ width: '484px' })}
          disableEscapeKeyDown={false}
          onClose={onClose}
          open={true}
        >
          <form>
            <DialogHeader style={{ flexDirection: 'column' }}>
              <DialogTitle>{dialogTitle}</DialogTitle>
            </DialogHeader>
            {addDeviceAttempt.status === 'failed' && (
              <Danger mt={2} width="100%">
                {addDeviceAttempt.statusText}
              </Danger>
            )}
            {fetchQrCodeAttempt.status === 'failed' && (
              <Danger mt={2} width="100%">
                {fetchQrCodeAttempt.statusText}
              </Danger>
            )}
            <DialogContent>
              <Flex
                flexDirection="column"
                justifyContent="center"
                alignItems="center"
                borderRadius={8}
                height="256px"
                p={3}
                mb={4}
                css={`
                  background: ${props => props.theme.colors.spotBackground[0]};
                `}
              >
                {mfaOption.value === 'otp' && (
                  <>
                    <Flex
                      height="168px"
                      justifyContent="center"
                      alignItems="center"
                    >
                      {fetchQrCodeAttempt.status === 'processing' && (
                        <Indicator />
                      )}
                      {fetchQrCodeAttempt.status === 'success' && (
                        <Image
                          src={`data:image/png;base64,${qrCode}`}
                          height="100%"
                          style={{
                            boxSizing: 'border-box',
                            border: '8px solid white',
                          }}
                        />
                      )}
                    </Flex>
                    <Text fontSize={1} textAlign="center" mt={2}>
                      Scan the QR Code with any authenticator app and enter the
                      generated code.{' '}
                      <Text color="text.slightlyMuted">
                        We recommend{' '}
                        <Link
                          href="https://authy.com/download/"
                          target="_blank"
                        >
                          Authy
                        </Link>
                        .
                      </Text>
                    </Text>
                  </>
                )}
                {mfaOption.value === 'webauthn' && (
                  <>
                    <Image src={secKeyGraphic} height="168px" />
                    <Text mt={3}>{hardwareInstructions}</Text>
                  </>
                )}
              </Flex>
              {restrictDeviceUsage !== 'passwordless' && (
                <Flex alignItems="center">
                  <FieldSelect
                    maxWidth="50%"
                    width="100%"
                    label="Two-factor Type"
                    data-testid="mfa-select"
                    value={mfaOption}
                    options={mfaOptions}
                    onChange={(o: MfaOption) => {
                      validator.reset();
                      onSetMfaOption(o);
                    }}
                    mr={3}
                    isDisabled={addDeviceAttempt.status === 'processing'}
                    elevated={true}
                  />
                  {mfaOption.value === 'otp' && (
                    <FieldInput
                      width="50%"
                      label="Authenticator Code"
                      rule={requiredToken}
                      inputMode="numeric"
                      autoComplete="one-time-code"
                      value={otpToken}
                      onChange={e => setOtpToken(e.target.value)}
                      placeholder="123 456"
                      readonly={addDeviceAttempt.status === 'processing'}
                    />
                  )}
                  {mfaOption.value === 'webauthn' &&
                    isPasswordlessEnabled &&
                    !restrictDeviceUsage && (
                      <FieldSelect
                        width="50%"
                        label="Allow Passwordless Login?"
                        value={usageOption}
                        options={deviceUsageOpts}
                        onChange={(o: DeviceusageOpt) => setUsageOption(o)}
                        isDisabled={addDeviceAttempt.status === 'processing'}
                        elevated={true}
                      />
                    )}
                </Flex>
              )}
              <FieldInput
                rule={requiredField('Device name is required')}
                label="Device Name"
                placeholder="Name"
                width="100%"
                autoFocus
                value={deviceName}
                type="text"
                onChange={e => setDeviceName(e.target.value)}
                readonly={addDeviceAttempt.status === 'processing'}
                mb={1}
              />
            </DialogContent>
            <DialogFooter>
              <ButtonPrimary
                size="large"
                width="45%"
                type="submit"
                onClick={e => validator.validate() && onSubmit(e)}
                disabled={addDeviceAttempt.status === 'processing'}
                mr={3}
              >
                Add device
              </ButtonPrimary>
              <ButtonSecondary size="large" width="30%" onClick={onClose}>
                Cancel
              </ButtonSecondary>
            </DialogFooter>
          </form>
        </Dialog>
      )}
    </Validation>
  );
}

type DeviceusageOpt = { value: DeviceUsage; label: string };
