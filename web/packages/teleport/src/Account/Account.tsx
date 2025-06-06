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

import React, { useCallback, useState } from 'react';
import { useHistory } from 'react-router';
import styled from 'styled-components';

import { Flex } from 'design';
import { Danger } from 'design/Alert';
import { ArrowBack } from 'design/Icon';
import { NotificationSeverity } from 'shared/components/Notification';
import { useStore } from 'shared/libs/stores';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { Redirect, Route, Switch } from 'teleport/components/Router';
import cfg from 'teleport/config';
import { PasswordState } from 'teleport/services/user';
import useTeleport from 'teleport/useTeleport';

import useManageDevices, {
  State as ManageDevicesState,
} from './ManageDevices/useManageDevices';
import {
  NotificationOutlet,
  NotificationProvider,
} from './NotificationContext';
import { Preferences } from './Preferences';
import { SecuritySettings } from './SecuritySettings';
import { SideNav } from './SideNav';

export interface EnterpriseComponentProps {
  // TODO(bl-nero): Consider moving the notifications to its own store and
  // unifying them between this screen and the unified resources screen.
  addNotification: (severity: NotificationSeverity, content: string) => void;
}

export interface AccountPageProps {
  enterpriseComponent?: React.ComponentType<EnterpriseComponentProps>;
  userTrustedDevicesComponent?: React.ComponentType;
}

export function AccountPage({
  enterpriseComponent,
  userTrustedDevicesComponent,
}: AccountPageProps) {
  const ctx = useTeleport();
  const storeUser = useStore(ctx.storeUser);
  const isSso = storeUser.isSso();
  const manageDevicesState = useManageDevices(ctx);

  const canAddPasskeys = cfg.isPasswordlessEnabled();
  const canAddMfa = cfg.isMfaEnabled();

  function onPasswordChange() {
    storeUser.setState({ passwordState: PasswordState.PASSWORD_STATE_SET });
  }

  return (
    <Account
      isSso={isSso}
      canAddPasskeys={canAddPasskeys}
      canAddMfa={canAddMfa}
      passwordState={storeUser.getPasswordState()}
      {...manageDevicesState}
      enterpriseComponent={enterpriseComponent}
      userTrustedDevicesComponent={userTrustedDevicesComponent}
      onPasswordChange={onPasswordChange}
    />
  );
}

export interface AccountProps extends ManageDevicesState, AccountPageProps {
  isSso: boolean;
  canAddPasskeys: boolean;
  canAddMfa: boolean;
  passwordState: PasswordState;
  onPasswordChange: () => void;
}

export function Account({
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
  mfaDisabled,
  createRestrictedTokenAttempt,
  userTrustedDevicesComponent: TrustedDeviceListComponent,
  passwordState,
  onPasswordChange: onPasswordChangeCb,
}: AccountProps) {
  const history = useHistory();

  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const stableSetErrorMessage = useCallback((message: string | null) => {
    setErrorMessage(message);
  }, []);

  return (
    <NotificationProvider>
      <Relative>
        <FeatureBox margin="auto">
          <FeatureHeader>
            <ArrowBack
              mr={2}
              size="large"
              color="text.main"
              onClick={history.goBack}
              style={{ cursor: 'pointer' }}
            />
            <FeatureHeaderTitle>Account Settings</FeatureHeaderTitle>
          </FeatureHeader>
          {!!errorMessage && (
            <Danger dismissible onDismiss={() => setErrorMessage(null)}>
              {errorMessage}
            </Danger>
          )}
          <Flex
            flexDirection="row"
            gap={4}
            maxWidth={'1440px'}
            margin={'0 auto'}
          >
            <Flex flexDirection="column" gap={1} width="16rem">
              <SideNav
                recoveryEnabled={EnterpriseComponent !== undefined}
                trustedDevicesEnabled={TrustedDeviceListComponent !== undefined}
              />
            </Flex>
            <Flex flexDirection="column" gap={4}>
              <Switch>
                <Route
                  exact
                  path={cfg.routes.account}
                  component={() => <Redirect to={cfg.routes.accountSecurity} />}
                />
                <Route
                  path={cfg.routes.accountSecurity}
                  component={() => (
                    <SecuritySettings
                      isSso={isSso}
                      canAddPasskeys={canAddPasskeys}
                      canAddMfa={canAddMfa}
                      passwordState={passwordState}
                      devices={devices}
                      onAddDevice={onAddDevice}
                      onRemoveDevice={onRemoveDevice}
                      onDeviceAdded={onDeviceAdded}
                      onDeviceRemoved={onDeviceRemoved}
                      deviceToRemove={deviceToRemove}
                      fetchDevicesAttempt={fetchDevicesAttempt}
                      addDeviceWizardVisible={addDeviceWizardVisible}
                      hideRemoveDevice={hideRemoveDevice}
                      closeAddDeviceWizard={closeAddDeviceWizard}
                      mfaDisabled={mfaDisabled}
                      createRestrictedTokenAttempt={
                        createRestrictedTokenAttempt
                      }
                      newDeviceUsage={newDeviceUsage}
                      enterpriseComponent={EnterpriseComponent}
                      userTrustedDevicesComponent={TrustedDeviceListComponent}
                      onPasswordChange={onPasswordChangeCb}
                    />
                  )}
                />
                <Route
                  path={cfg.routes.accountPreferences}
                  component={() => (
                    <Preferences setErrorMessage={stableSetErrorMessage} />
                  )}
                />
              </Switch>
            </Flex>
          </Flex>
          <NotificationOutlet />
        </FeatureBox>
      </Relative>
    </NotificationProvider>
  );
}

const Relative = styled.div`
  position: relative;
`;
