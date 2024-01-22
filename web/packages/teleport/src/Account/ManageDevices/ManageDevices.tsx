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

import React from 'react';
import { Box, Indicator, ButtonPrimary } from 'design';
import { Danger } from 'design/Alert';

import useTeleport from 'teleport/useTeleport';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import MfaDeviceList, { RemoveDialog } from 'teleport/components/MfaDeviceList';

import ReAuthenticate from 'teleport/components/ReAuthenticate';

import { MFAChallengeScope } from 'teleport/services/auth/auth';

import AddDevice from './AddDevice';
import useManageDevices, { State } from './useManageDevices';

export default function Container() {
  const ctx = useTeleport();
  const state = useManageDevices(ctx);
  return <ManageDevices {...state} />;
}

export function ManageDevices({
  token,
  setToken,
  onAddDevice,
  onRemoveDevice,
  createRestrictedTokenAttempt,
  devices,
  fetchDevices,
  fetchDevicesAttempt,
  removeDevice,
  deviceToRemove,
  isAddDeviceVisible,
  isReAuthenticateVisible,
  isRemoveDeviceVisible,
  hideReAuthenticate,
  hideAddDevice,
  hideRemoveDevice,
  mfaDisabled,
}: State) {
  return (
    <>
      <FeatureBox style={{ width: '904px', padding: 0 }}>
        <FeatureHeader
          alignItems="center"
          justifyContent="space-between"
          border="none"
        >
          <FeatureHeaderTitle>Two-Factor Devices</FeatureHeaderTitle>
          <ButtonPrimary
            onClick={onAddDevice}
            disabled={
              createRestrictedTokenAttempt.status === 'processing' ||
              mfaDisabled
            }
            title={mfaDisabled ? 'Two-factor authentication is disabled' : ''}
          >
            Add two-factor device
          </ButtonPrimary>
        </FeatureHeader>
        {fetchDevicesAttempt.status === 'processing' && (
          <Box textAlign="center">
            <Indicator />
          </Box>
        )}
        {createRestrictedTokenAttempt.status === 'failed' && (
          <Danger mb={3}>{createRestrictedTokenAttempt.statusText}</Danger>
        )}
        {fetchDevicesAttempt.status === 'failed' && (
          <Danger mb={3}>{fetchDevicesAttempt.statusText}</Danger>
        )}
        {fetchDevicesAttempt.status === 'success' && (
          <MfaDeviceList
            devices={devices}
            remove={onRemoveDevice}
            mfaDisabled={mfaDisabled}
            style={{ maxWidth: '100%' }}
            isSearchable
          />
        )}
      </FeatureBox>
      {isReAuthenticateVisible && (
        <ReAuthenticate
          onAuthenticated={setToken}
          onClose={hideReAuthenticate}
          actionText="registering a new device"
          challengeScope={MFAChallengeScope.USER_SESSION}
        />
      )}
      {isAddDeviceVisible && (
        <AddDevice
          fetchDevices={fetchDevices}
          token={token}
          onClose={hideAddDevice}
        />
      )}
      {isRemoveDeviceVisible && (
        <RemoveDialog
          name={deviceToRemove.name}
          onRemove={removeDevice}
          onClose={hideRemoveDevice}
        />
      )}
    </>
  );
}
