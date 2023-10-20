/*
Copyright 2021 Gravitational, Inc.

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
