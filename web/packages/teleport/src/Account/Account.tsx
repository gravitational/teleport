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

import React, {useContext, useState} from 'react';
import { Box, Flex, Text } from 'design';
import styled, { useTheme } from 'styled-components';
import { Attempt } from 'shared/hooks/useAttemptNext';
import * as Icon from 'design/Icon';
import { Notification, NotificationItem } from 'shared/components/Notification';

import Option from "shared/components/Select"

import useTeleport from 'teleport/useTeleport';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import ReAuthenticate from 'teleport/components/ReAuthenticate';
import { RemoveDialog } from 'teleport/components/MfaDeviceList';

import cfg from 'teleport/config';

import { DeviceUsage } from 'teleport/services/auth';

import { AuthDeviceList } from './ManageDevices/AuthDeviceList/AuthDeviceList';
import useManageDevices, {
  State as ManageDevicesState,
} from './ManageDevices/useManageDevices';
import { ActionButtonPrimary, ActionButtonSecondary, Header } from './Header';
import { PasswordBox } from './PasswordBox';
import { AddAuthDeviceWizard } from './ManageDevices/AddAuthDeviceWizard';
import {SingleRowBox} from "design/MultiRowBox";
import Select from "shared/components/Select";
import {UserContext} from "teleport/User/UserContext";

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

  let ctx = useContext(UserContext);

  let layouts = {
    0: 'System',
    0x00000401:'Arabic (101)',
    0x00000402:'Bulgarian',
    0x00000404:'Chinese (Traditional) - US Keyboard',
    0x00000405:'Czech',
    0x00000406:'Danish',
    0x00000407:'German',
    0x00000408:'Greek',
    0x00000409:'US',
    0x0000040a:'Spanish',
    0x0000040b:'Finnish',
    0x0000040c:'French',
    0x0000040d:'Hebrew',
    0x0000040e:'Hungarian',
    0x0000040f:'Icelandic',
    0x00000410:'Italian',
    0x00000411:'Japanese',
    0x00000412:'Korean',
    0x00000413:'Dutch',
    0x00000414:'Norwegian',
    0x00000415:'Polish (Programmers)',
    0x00000416:'Portuguese (Brazilian ABNT)',
    0x00000418:'Romanian (Legacy)',
    0x00000419:'Russian',
    0x0000041a:'Croatian',
    0x0000041b:'Slovak',
    0x0000041c:'Albanian',
    0x0000041d:'Swedish',
    0x0000041e:'Thai Kedmanee',
    0x0000041f:'Turkish Q',
    0x00000420:'Urdu',
    0x00000422:'Ukrainian',
    0x00000423:'Belarusian',
    0x00000424:'Slovenian',
    0x00000425:'Estonian',
    0x00000426:'Latvian',
    0x00000427:'Lithuanian IBM',
    0x00000428:'Tajik',
    0x00000429:'Persian',
    0x0000042a:'Vietnamese',
    0x0000042b:'Armenian Eastern',
    0x0000042c:'Azeri Latin',
    0x0000042e:'Sorbian Standard',
    0x0000042f:'Macedonian (FYROM)',
    0x00000437:'Georgian',
    0x00000438:'Faeroese',
    0x00000439:'Devanagari-INSCRIPT',
    0x0000043a:'Maltese 47-Key',
    0x0000043b:'Norwegian with Sami',
    0x0000043f:'Kazakh',
    0x00000440:'Kyrgyz Cyrillic',
    0x00000442:'Turkmen',
    0x00000444:'Tatar',
    0x00000445:'Bengali',
    0x00000446:'Punjabi',
    0x00000447:'Gujarati',
    0x00000448:'Oriya',
    0x00000449:'Tamil',
    0x0000044a:'Telugu',
    0x0000044b:'Kannada',
    0x0000044c:'Malayalam',
    0x0000044d:'ASSAMESE - INSCRIPT',
    0x0000044e:'Marathi',
    0x00000450:'Mongolian Cyrillic',
    0x00000451: "Tibetan (People's Republic of China)",
    0x00000452:'United Kingdom Extended',
    0x00000453:'Khmer',
    0x00000454:'Lao',
    0x0000045a:'Syriac',
    0x0000045b:'Sinhala',
    0x00000461:'Nepali',
    0x00000463:'Pashto (Afghanistan)',
    0x00000465:'Divehi Phonetic',
    0x0000046d:'Bashkir',
    0x0000046e:'Luxembourgish',
    0x0000046f:'Greenlandic',
    0x00000480:'Uighur',
    0x00000481:'Maori',
    0x00000485:'Yakut',
    0x00000804:'Chinese (Simplified) - US Keyboard',
    0x00000807:'Swiss German',
    0x00000809:'United Kingdom',
    0x0000080a:'Latin American',
    0x0000080c:'Belgian French',
    0x00000813:'Belgian (Period)',
    0x00000816:'Portuguese',
    0x0000081a:'Serbian (Latin)',
    0x0000082c:'Azeri Cyrillic',
    0x0000083b:'Swedish with Sami',
    0x00000843:'Uzbek Cyrillic',
    0x00000850:'Mongolian (Mongolian Script)',
    0x0000085d:'Inuktitut - Latin',
    0x00000c0c:'Canadian French (Legacy)',
    0x00000c1a:'Serbian (Cyrillic)',
    0x00001009:'Canadian French',
    0x0000100c:'Swiss French',
    0x00001809:'Irish',
    0x0000201a:'Bosnian (Cyrillic)',
    0x00010401:'Arabic (102)',
    0x00010402:'Bulgarian (Latin)',
    0x00010405:'Czech (QWERTY)',
    0x00010407:'German (IBM)',
    0x00010408:'Greek (220)',
    0x00010409:'United States - Dvorak',
    0x0001040a:'Spanish Variation',
    0x0001040e:'Hungarian 101-key',
    0x00010410:'Italian (142)',
    0x00010415:'Polish (214)',
    0x00010416:'Portuguese (Brazilian ABNT2)',
    0x00010418:'Romanian (Standard)',
    0x00010419:'Russian (Typewriter)',
    0x0001041b:'Slovak (QWERTY)',
    0x0001041e:'Thai Pattachote',
    0x0001041f:'Turkish F',
    0x00010426:'Latvian (QWERTY)',
    0x00010427:'Lithuanian',
    0x0001042b:'Armenian Western',
    0x0001042e:'Sorbian Extended',
    0x0001042f:'Macedonian (FYROM) - Standard',
    0x00010437:'Georgian (QWERTY)',
    0x00010439:'Hindi Traditional',
    0x0001043a:'Maltese 48-key',
    0x0001043b:'Sami Extended Norway',
    0x00010445:'Bengali - INSCRIPT (Legacy)',
    0x0001045a:'Syriac Phonetic',
    0x0001045b:'Sinhala - wij 9',
    0x0001045d:'Inuktitut - Naqittaut',
    0x00010465:'Divehi Typewriter',
    0x0001080c:'Belgian (Comma)',
    0x0001083b:'Finnish with Sami',
    0x00011009:'Canadian Multilingual Standard',
    0x00011809:'Gaelic',
    0x00020401:'Arabic (102) AZERTY',
    0x00020402:'Bulgarian (phonetic layout)',
    0x00020405:'Czech Programmers',
    0x00020408:'Greek (319)',
    0x00020409:'United States - International',
    0x00020418:'Romanian (Programmers)',
    0x0002041e:'Thai Kedmanee (non-ShiftLock)',
    0x00020422:'Ukrainian (Enhanced)',
    0x00020427:'Lithuanian New',
    0x00020437:'Georgian (Ergonomic)',
    0x00020445:'Bengali - INSCRIPT',
    0x0002083b:'Sami Extended Finland-Sweden',
    0x00030402:'Bulgarian (phonetic layout)',
    0x00030408:'Greek (220) Latin',
    0x00030409:'United States-Devorak for left hand',
    0x0003041e:'Thai Pattachote (non-ShiftLock)',
    0x00040408:'Greek (319) Latin',
    0x00040409:'United States-Dvorak for right hand',
    0x00050409:'Greek Latin',
    0x00060408:'Greek Polytonic',
  }

  let layout = ctx.preferences.keyboardLayout;
  let value: Option<any, any> = {label: layouts[layout], value: layout};
  let options: Option<any, any>[] = Object.keys(layouts).map(k=> { return {label: layouts[k], value: parseInt(k)}});

  return (
    <Relative>
      <FeatureBox>
        <FeatureHeader>
          <FeatureHeaderTitle>Account Settings</FeatureHeaderTitle>
        </FeatureHeader>
        <Flex flexDirection="column" gap={4}>
          <Box>
            <SingleRowBox>
              <Header
                  title="Keyboard layout"
                  description="Keyboard layout used by Windows Desktop sessions"
                  icon={<Icon.Edit/>}
                  actions={
                    <Box width="210px">
                      <Select
                          onChange={selected => {
                            if (Array.isArray(selected)) {
                              selected = selected[0];
                            }
                            console.log("value", selected.value)
                            ctx.updatePreferences({
                              keyboardLayout: selected.value
                            }).then(() => value = selected);
                          }}
                          value={value}
                          options={options}/>
                    </Box>}/>
            </SingleRowBox>
          </Box>
          <Box>
            <AuthDeviceList
              header={
                <PasskeysHeader
                  empty={devices.length === 0}
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
          {isReAuthenticateVisible && (
            <ReAuthenticate
              onAuthenticated={setToken}
              onClose={hideReAuthenticate}
              actionText="registering a new device"
            />
          )}
          {EnterpriseComponent && (
            <EnterpriseComponent addNotification={addNotification} />
          )}
        </Flex>
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

/**
 * Renders a simple header for non-empty list of passkeys, and a more
 * encouraging CTA if there are no passkeys.
 */
function PasskeysHeader({
  empty,
  fetchDevicesAttempt,
  disableAddPasskey,
  onAddDevice,
}: {
  empty: boolean;
  fetchDevicesAttempt: Attempt;
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
        <Text typography="h4">Passwordless sign-in using Passkeys</Text>
        <Text
          typography="body1"
          color={theme.colors.text.slightlyMuted}
          textAlign="center"
          mb={3}
        >
          Passkeys are a password replacement that validates your identity using
          touch, facial recognition, a device password, or a PIN.
        </Text>
        {button}
      </Flex>
    );
  }

  return (
    <Header
      title="Passkeys"
      description="Enable secure passwordless sign-in using
                fingerprint or facial recognition, a one-time code, or
                a device password."
      icon={<Icon.Key />}
      showIndicator={fetchDevicesAttempt.status === 'processing'}
      actions={button}
    />
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
