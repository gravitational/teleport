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

import React, { useState } from 'react';
import { Box } from 'design';
import styled from 'styled-components';
import { Attempt } from 'shared/hooks/useAttemptNext';
import * as Icon from 'design/Icon';
import { Notification, NotificationItem } from 'shared/components/Notification';

import useTeleport from 'teleport/useTeleport';
import { FeatureBox } from 'teleport/components/Layout';
import ReAuthenticate from 'teleport/components/ReAuthenticate';
import { RemoveDialog } from 'teleport/components/MfaDeviceList';

import { MfaChallengeScope } from 'teleport/services/auth/auth';

import cfg from 'teleport/config';

import { AuthDeviceList } from './ManageDevices/AuthDeviceList/AuthDeviceList';
import useManageDevices, {
  State as ManageDevicesState,
} from './ManageDevices/useManageDevices';
import { ActionButton, Header } from './Header';
import { PasswordBox } from './PasswordBox';
import { AddAuthDeviceWizard } from './ManageDevices/AddAuthDeviceWizard';

export interface EnterpriseComponentProps {
  // TODO(bl-nero): Consider moving the notifications to its own store and
  // unifying them between this screen and the unified resources screen.
  addNotification: (
    severity: NotificationItem['severity'],
    content: string
  ) => void;
}

export interface AccountPageProps {
  enterpriseComponent?: React.ComponentType<EnterpriseComponentProps>;
}

export function AccountPage({ enterpriseComponent }: AccountPageProps) {
  const ctx = useTeleport();
  const isSso = ctx.storeUser.isSso();
  const manageDevicesState = useManageDevices(ctx);

  const canAddPasskeys = cfg.isPasswordlessEnabled();
  const canAddMfa = cfg.isMfaEnabled();

  return (
    <Account
      isSso={isSso}
      canAddPasskeys={canAddPasskeys}
      canAddMfa={canAddMfa}
      {...manageDevicesState}
      enterpriseComponent={enterpriseComponent}
    />
  );
}

export interface AccountProps extends ManageDevicesState, AccountPageProps {
  isSso: boolean;
  canAddPasskeys: boolean;
  canAddMfa: boolean;
}

export function Account({
  devices,
  token,
  setToken,
  onAddDevice,
  onRemoveDevice,
  onDeviceAdded,
  deviceToRemove,
  removeDevice,
  fetchDevicesAttempt,
  createRestrictedTokenAttempt,
  isReAuthenticateVisible,
  isRemoveDeviceVisible,
  addDeviceWizardVisible,
  hideReAuthenticate,
  hideRemoveDevice,
  closeAddDeviceWizard,
  isSso,
  canAddMfa,
  canAddPasskeys,
  enterpriseComponent: EnterpriseComponent,
  newDeviceUsage,
}: AccountProps) {
  const passkeys = devices.filter(d => d.usage === 'passwordless');
  const mfaDevices = devices.filter(d => d.usage === 'mfa');
  const disableAddDevice =
    createRestrictedTokenAttempt.status === 'processing' ||
    fetchDevicesAttempt.status !== 'success';
  const disableAddPasskey = disableAddDevice || !canAddPasskeys;
  const disableAddMfa = disableAddDevice || !canAddMfa;

  const [notifications, setNotifications] = useState<NotificationItem[]>([]);
  const [prevFetchStatus, setPrevFetchStatus] = useState<Attempt['status']>('');
  const [prevTokenStatus, setPrevTokenStatus] = useState<Attempt['status']>('');

  function addNotification(
    severity: NotificationItem['severity'],
    content: string
  ) {
    setNotifications(n => [
      ...n,
      {
        id: crypto.randomUUID(),
        severity,
        content,
      },
    ]);
  }

  function removeNotification(id: string) {
    setNotifications(n => n.filter(item => item.id !== id));
  }

  // TODO(bl.nero): Modify `useManageDevices` and export callbacks from there instead.
  if (prevFetchStatus !== fetchDevicesAttempt.status) {
    setPrevFetchStatus(fetchDevicesAttempt.status);
    if (fetchDevicesAttempt.status === 'failed') {
      addNotification('error', fetchDevicesAttempt.statusText);
    }
  }

  if (prevTokenStatus !== createRestrictedTokenAttempt.status) {
    setPrevTokenStatus(createRestrictedTokenAttempt.status);
    if (createRestrictedTokenAttempt.status === 'failed') {
      addNotification('error', createRestrictedTokenAttempt.statusText);
    }
  }

  function onPasswordChange() {
    addNotification('info', 'Your password has been changed.');
  }

  function onAddDeviceSuccess() {
    const message =
      newDeviceUsage === 'passwordless'
        ? 'Passkey successfully saved.'
        : 'MFA device successfully saved.';
    addNotification('info', message);
    onDeviceAdded();
  }

  return (
    <Relative>
      <FeatureBox gap={4} mt={4}>
        <Box>
          <AuthDeviceList
            header={
              <Header
                title="Passkeys"
                description="Enable secure passwordless sign-in using
                fingerprint or facial recognition, a one-time code, or
                a device password."
                icon={<Icon.Key />}
                showIndicator={fetchDevicesAttempt.status === 'processing'}
                actions={
                  <ActionButton
                    disabled={disableAddPasskey}
                    title={
                      disableAddPasskey
                        ? 'Passwordless authentication is disabled'
                        : ''
                    }
                    onClick={() => onAddDevice('passwordless')}
                  >
                    <Icon.Add size={20} />
                    Add a Passkey
                  </ActionButton>
                }
              />
            }
            deviceTypeColumnName="Passkey Type"
            devices={passkeys}
            onRemove={onRemoveDevice}
          />
        </Box>
        {!isSso && (
          <PasswordBox
            changeDisabled={
              createRestrictedTokenAttempt.status === 'processing'
            }
            devices={devices}
            onPasswordChange={onPasswordChange}
          />
        )}
        <Box>
          <AuthDeviceList
            header={
              <Header
                title="Multi-factor Authentication"
                description="Provide secondary authentication when signing in
                with a password. Unlike passkeys, multi-factor methods do not
                enable passwordless sign-in."
                icon={<Icon.ShieldCheck />}
                showIndicator={fetchDevicesAttempt.status === 'processing'}
                actions={
                  <ActionButton
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
                  </ActionButton>
                }
              />
            }
            deviceTypeColumnName="MFA Type"
            devices={mfaDevices}
            onRemove={onRemoveDevice}
          />
        </Box>
        {isReAuthenticateVisible && (
          <ReAuthenticate
            onAuthenticated={setToken}
            onClose={hideReAuthenticate}
            actionText="registering a new device"
            challengeScope={MfaChallengeScope.MANAGE_DEVICES}
          />
        )}
        {EnterpriseComponent && (
          <EnterpriseComponent addNotification={addNotification} />
        )}
      </FeatureBox>

      {isRemoveDeviceVisible && (
        <RemoveDialog
          name={deviceToRemove.name}
          onRemove={removeDevice}
          onClose={hideRemoveDevice}
        />
      )}

      {addDeviceWizardVisible && (
        <AddAuthDeviceWizard
          usage={newDeviceUsage}
          auth2faType={cfg.getAuth2faType()}
          privilegeToken={token}
          onClose={closeAddDeviceWizard}
          onSuccess={onAddDeviceSuccess}
        />
      )}

      {/* Note: Although notifications appear on top, we deliberately place the
          container on the bottom to avoid manipulating z-index. The stacking
          context from one of the buttons appears on top otherwise.

          TODO(bl-nero): Consider reusing the Notifications component from
          Teleterm. */}
      <NotificationContainer>
        {notifications.map(item => (
          <Notification
            style={{ marginBottom: '12px' }}
            key={item.id}
            item={item}
            Icon={notificationIcon(item.severity)}
            getColor={notificationColor(item.severity)}
            onRemove={() => removeNotification(item.id)}
            isAutoRemovable={item.severity === 'info'}
          />
        ))}
      </NotificationContainer>
    </Relative>
  );
}

const NotificationContainer = styled.div`
  position: absolute;
  top: ${props => props.theme.space[2]}px;
  right: ${props => props.theme.space[5]}px;
`;

const Relative = styled.div`
  position: relative;
`;

function notificationIcon(severity: NotificationItem['severity']) {
  switch (severity) {
    case 'info':
      return Icon.Info;
    case 'warn':
      return Icon.Warning;
    case 'error':
      return Icon.WarningCircle;
  }
}

function notificationColor(severity: NotificationItem['severity']) {
  switch (severity) {
    case 'info':
      return theme => theme.colors.info;
    case 'warn':
      return theme => theme.colors.warning.main;
    case 'error':
      return theme => theme.colors.error.main;
  }
}
