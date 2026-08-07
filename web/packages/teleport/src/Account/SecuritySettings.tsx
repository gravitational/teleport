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

import { Box, Flex } from 'design';
import * as Icon from 'design/Icon';
import { useToastNotifications } from 'shared/components/ToastNotification';
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
import { PasswordBox } from './PasswordBox';
import { Headings } from './SideNav';
import { AuthMethodState, StatePill } from './StatePill';

export interface SecuritySettingsProps extends AccountProps {}

/**
 * For use by the account setting's side nav to determine which headings to show.
 * @returns Array of headings to show in the side nav.
 */
export function securityHeadings(): Headings {
  const ctx = useTeleport();
  const storeUser = useStore(ctx.storeUser);
  const isSso = storeUser.isSso();

  let headings = [
    { name: 'Passkeys and Multi-factor Authentication', id: 'auth-devices' },
  ] as Headings;

  if (!isSso) {
    headings.push({ name: 'Password', id: 'password' });
  }

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
  const toastNotification = useToastNotifications();

  const hasPasskeys = devices.some(d => d.usage === 'passwordless');
  const hasMfaDevices = devices.some(d => d.usage === 'mfa');
  const disableAddPasskey = !canAddPasskeys;
  const disableAddMfa = !canAddMfa;

  let mfaPillState = undefined;
  if (fetchDevicesAttempt.status !== 'processing') {
    mfaPillState = canAddMfa && hasMfaDevices ? 'active' : 'inactive';
  }

  let passkeysPillState: AuthMethodState | undefined = undefined;
  if (!canAddPasskeys) {
    passkeysPillState = 'disabled';
  } else if (fetchDevicesAttempt.status !== 'processing') {
    passkeysPillState = hasPasskeys ? 'active' : 'inactive';
  }

  const [prevFetchStatus, setPrevFetchStatus] = useState<Attempt['status']>('');

  // TODO(bl.nero): Modify `useManageDevices` and export callbacks from there instead.
  if (prevFetchStatus !== fetchDevicesAttempt.status) {
    setPrevFetchStatus(fetchDevicesAttempt.status);
    if (fetchDevicesAttempt.status === 'failed') {
      toastNotification.add({
        severity: 'error',
        content: fetchDevicesAttempt.statusText,
      });
    }
  }

  function onPasswordChange() {
    toastNotification.add({
      severity: 'info',
      content: {
        title: 'Your password has been changed.',
      },
    });
    onPasswordChangeCb();
  }

  function onAddDeviceSuccess() {
    const message =
      newDeviceUsage === 'passwordless'
        ? 'Passkey successfully saved.'
        : 'MFA method successfully saved.';
    toastNotification.add({ severity: 'info', content: { title: message } });
    onDeviceAdded();
  }

  function onDeleteDeviceSuccess() {
    const message =
      deviceToRemove.usage === 'passwordless'
        ? 'Passkey successfully deleted.'
        : 'MFA method successfully deleted.';
    toastNotification.add({
      severity: 'info',
      content: { title: message },
    });
    onDeviceRemoved();
  }
  return (
    <>
      <Box id="auth-devices" data-testid="device-list">
        <AuthDeviceList
          header={
            <Flex flexDirection="column" gap={3}>
              <PasskeysHeader
                empty={!hasPasskeys}
                state={passkeysPillState}
                disableAddPasskey={disableAddPasskey}
                onAddDevice={onAddDevice}
              />
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
            </Flex>
          }
          devices={devices}
          onRemove={onRemoveDevice}
          attempt={fetchDevicesAttempt}
          passkeysEnabled={canAddPasskeys}
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
      {EnterpriseComponent && (
        <div id="recovery-code">
          <EnterpriseComponent addNotification={toastNotification.add} />
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
  state,
  disableAddPasskey,
  onAddDevice,
}: {
  empty: boolean;
  state: AuthMethodState | undefined;
  disableAddPasskey: boolean;
  onAddDevice: (usage: DeviceUsage) => void;
}) {
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

  const title = empty ? 'Passwordless sign-in using Passkeys' : 'Passkeys';

  const description = empty
    ? 'Passkeys are a password replacement that validates your identity using touch, facial recognition, a device password, or a PIN.'
    : 'Enable secure passwordless sign-in using fingerprint or facial recognition, a one-time code, or a device password.';

  return (
    <Header
      title={
        <Flex gap={2} alignItems="center">
          {title}
          <StatePill data-testid="passwordless-state-pill" state={state} />
        </Flex>
      }
      description={description}
      icon={<Icon.Key />}
      actions={button}
    />
  );
}
