/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
import { useState } from 'react';
import styled, { useTheme } from 'styled-components';

import { Box, Flex, H2, Indicator, Subtitle2 } from 'design';
import * as Icon from 'design/Icon';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { useStore } from 'shared/libs/stores';

import cfg from 'teleport/config';
import { DeviceUsage } from 'teleport/services/mfa';
import useTeleport from 'teleport/useTeleport';

import { AccountProps } from './Account';
import { ActionButtonPrimary, ActionButtonSecondary, Header } from './Header';
import { AuthDeviceList } from './ManageDevices/AuthDeviceList/AuthDeviceList';
import {
  AddAuthDeviceWizard,
  DeleteAuthDeviceWizard,
} from './ManageDevices/wizards';
import { useNotification } from './NotificationContext';
import { PasswordBox } from './PasswordBox';
import { Headings } from './SideNav';
import { StatePill } from './StatePill';

export interface SecuritySettingsProps extends AccountProps {}

/**
 * For use by the account setting's side nav to determine which headings to show.
 * @returns Array of headings to show in the side nav.
 */
export function securityHeadings(): Headings {
  const ctx = useTeleport();
  const storeUser = useStore(ctx.storeUser);
  const isSso = storeUser.isSso();

  let headings = [{ name: 'Passkey', id: 'passkey' }] as Headings;

  if (!isSso) {
    headings.push({ name: 'Password', id: 'password' });
  }

  headings.push({ name: 'Multi-factor Authentication', id: 'mfa' });

  return headings;
}

export function SecuritySettings({
  devices,
  onAddDevice,
  onRemoveDevice,
  onDeviceAdded,
  onDeviceRemoved,
  deviceToRemove,
  fetchDevicesAttempt,
  addDeviceWizardVisible,
  hideRemoveDevice,
  closeAddDeviceWizard,
  isSso,
  canAddMfa,
  canAddPasskeys,
  enterpriseComponent: EnterpriseComponent,
  newDeviceUsage,
  userTrustedDevicesComponent: TrustedDeviceListComponent,
  passwordState,
  onPasswordChange: onPasswordChangeCb,
}: SecuritySettingsProps) {
  const { addNotification } = useNotification();

  const passkeys = devices.filter(d => d.usage === 'passwordless');
  const mfaDevices = devices.filter(d => d.usage === 'mfa');
  const disableAddPasskey = !canAddPasskeys;
  const disableAddMfa = !canAddMfa;

  let mfaPillState = undefined;
  if (fetchDevicesAttempt.status !== 'processing') {
    mfaPillState = canAddMfa && mfaDevices.length > 0 ? 'active' : 'inactive';
  }

  const [prevFetchStatus, setPrevFetchStatus] = useState<Attempt['status']>('');

  // TODO(bl.nero): Modify `useManageDevices` and export callbacks from there instead.
  if (prevFetchStatus !== fetchDevicesAttempt.status) {
    setPrevFetchStatus(fetchDevicesAttempt.status);
    if (fetchDevicesAttempt.status === 'failed') {
      addNotification('error', fetchDevicesAttempt.statusText);
    }
  }

  function onPasswordChange() {
    addNotification('info', {
      title: 'Your password has been changed.',
      isAutoRemovable: true,
    });
    onPasswordChangeCb();
  }

  function onAddDeviceSuccess() {
    const message =
      newDeviceUsage === 'passwordless'
        ? 'Passkey successfully saved.'
        : 'MFA method successfully saved.';
    addNotification('info', { title: message, isAutoRemovable: true });
    onDeviceAdded();
  }

  function onDeleteDeviceSuccess() {
    const message =
      deviceToRemove.usage === 'passwordless'
        ? 'Passkey successfully deleted.'
        : 'MFA method successfully deleted.';
    addNotification('info', { title: message, isAutoRemovable: true });
    onDeviceRemoved();
  }
  return (
    <>
      <Box data-testid="passkey-list" id="passkey">
        <AuthDeviceList
          header={
            <PasskeysHeader
              empty={passkeys.length === 0}
              passkeysEnabled={canAddPasskeys}
              disableAddPasskey={disableAddPasskey}
              fetchDevicesAttempt={fetchDevicesAttempt}
              onAddDevice={onAddDevice}
            />
          }
          deviceTypeColumnName="Passkey Type"
          devices={passkeys}
          onRemove={onRemoveDevice}
        />
      </Box>
      {!isSso && (
        <div id="password">
          <PasswordBox
            devices={devices}
            passwordState={passwordState}
            onPasswordChange={onPasswordChange}
          />
        </div>
      )}
      <Box data-testid="mfa-list" id="mfa">
        <AuthDeviceList
          header={
            <Header
              title={
                <Flex gap={2} alignItems="center">
                  Multi-factor Authentication
                  <StatePill
                    data-testid="mfa-state-pill"
                    state={mfaPillState}
                  />
                </Flex>
              }
              description="Provide secondary authentication when signing in
                      with a password. Unlike passkeys, multi-factor methods do
                      not enable passwordless sign-in."
              icon={<Icon.ShieldCheck />}
              showIndicator={fetchDevicesAttempt.status === 'processing'}
              actions={
                <ActionButtonSecondary
                  disabled={disableAddMfa}
                  title={
                    disableAddMfa
                      ? 'Multi-factor authentication is disabled'
                      : ''
                  }
                  onClick={() => onAddDevice('mfa')}
                >
                  <Icon.Add size={20} />
                  Add MFA
                </ActionButtonSecondary>
              }
            />
          }
          deviceTypeColumnName="MFA Type"
          devices={mfaDevices}
          onRemove={onRemoveDevice}
        />
      </Box>
      {EnterpriseComponent && (
        <div id="recovery-code">
          <EnterpriseComponent addNotification={addNotification} />
        </div>
      )}
      {TrustedDeviceListComponent && (
        <div id="trusted-devices">
          <TrustedDeviceListComponent />
        </div>
      )}

      {addDeviceWizardVisible && (
        <AddAuthDeviceWizard
          usage={newDeviceUsage}
          auth2faType={cfg.getAuth2faType()}
          onClose={closeAddDeviceWizard}
          onSuccess={onAddDeviceSuccess}
        />
      )}

      {deviceToRemove && (
        <DeleteAuthDeviceWizard
          deviceToDelete={deviceToRemove}
          onClose={hideRemoveDevice}
          onSuccess={onDeleteDeviceSuccess}
        />
      )}
    </>
  );
}

/**
 * Renders a simple header for non-empty list of passkeys, and a more
 * encouraging CTA if there are no passkeys.
 */
function PasskeysHeader({
  empty,
  fetchDevicesAttempt,
  passkeysEnabled,
  disableAddPasskey,
  onAddDevice,
}: {
  empty: boolean;
  fetchDevicesAttempt: Attempt;
  passkeysEnabled: boolean;
  disableAddPasskey: boolean;
  onAddDevice: (usage: DeviceUsage) => void;
}) {
  const theme = useTheme();

  const ActionButton = empty ? ActionButtonPrimary : ActionButtonSecondary;
  const button = (
    <ActionButton
      disabled={disableAddPasskey}
      title={disableAddPasskey ? 'Passwordless authentication is disabled' : ''}
      onClick={() => onAddDevice('passwordless')}
    >
      <Icon.Add size={20} />
      Add a Passkey
    </ActionButton>
  );

  if (empty) {
    return (
      <Flex flexDirection="column" alignItems="center">
        <Box
          bg={theme.colors.interactive.tonal.neutral[0]}
          lineHeight={0}
          p={2}
          borderRadius={3}
          mb={3}
        >
          <Icon.Key />
        </Box>
        <H2 mb={1}>Passwordless sign-in using Passkeys</H2>
        <Subtitle2
          color={theme.colors.text.slightlyMuted}
          textAlign="center"
          mb={3}
        >
          Passkeys are a password replacement that validates your identity using
          touch, facial recognition, a device password, or a PIN.
        </Subtitle2>
        <RelativeBox>
          {fetchDevicesAttempt.status === 'processing' && (
            // This trick allows us to maintain center alignment of the button
            // and display it along with the indicator.
            <BoxToTheRight mr={3} data-testid="indicator-wrapper">
              <Indicator size={40} delay="none" />
            </BoxToTheRight>
          )}
          {button}
        </RelativeBox>
      </Flex>
    );
  }

  return (
    <Header
      title={
        <Flex gap={2} alignItems="center">
          Passkeys
          <StatePill
            data-testid="passwordless-state-pill"
            state={passkeysEnabled ? 'active' : 'inactive'}
          />
        </Flex>
      }
      description="Enable secure passwordless sign-in using
                fingerprint or facial recognition, a one-time code, or
                a device password."
      icon={<Icon.Key />}
      showIndicator={fetchDevicesAttempt.status === 'processing'}
      actions={button}
    />
  );
}

const RelativeBox = styled(Box)`
  position: relative;
`;

/** A box that is displayed to the right where it normally would be. */
const BoxToTheRight = styled(Box)`
  position: absolute;
  right: 100%;
`;
