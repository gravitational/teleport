/*
Copyright 2023 Gravitational, Inc.

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
import { NewLockView } from 'teleport/LocksV2/NewLock/NewLock';
import {
  LockResourceOption,
  baseResourceKindOpts,
  LockResourceKind,
} from 'teleport/LocksV2/NewLock/common';
import {
  HybridListProps,
  SimpleListProps,
} from 'teleport/LocksV2/NewLock/ResourceList/common';

import useTeleportE from 'e-teleport/useTeleportE';
import { AccessRequest } from 'e-teleport/services/workflow';
import { TrustedDevice } from 'e-teleport/services/devices/types';

import { AccessRequests } from './SimpleList/AccessRequests';
import { TrustedDevices } from './HybridList/TrustedDevices';

export function NewLock() {
  const ctx = useTeleportE();

  function getFetchFn(resourceType: LockResourceKind) {
    if (resourceType === 'device') {
      return ctx.deviceService.fetchDevices;
    }
    if (resourceType === 'access_request') {
      return ctx.workflowService.fetchAccessRequests;
    }
  }

  function getTableForSimpleList(
    resourceKind: LockResourceKind,
    resources: any[],
    listProps: SimpleListProps
  ) {
    if (resourceKind === 'access_request') {
      return (
        <AccessRequests
          {...listProps}
          requests={resources as AccessRequest[]}
        />
      );
    }
  }

  function getTableForHybridList(
    resourceKind: LockResourceKind,
    resources: any[],
    listProps: HybridListProps
  ) {
    if (resourceKind === 'device') {
      return (
        <TrustedDevices {...listProps} devices={resources as TrustedDevice[]} />
      );
    }
  }

  return (
    <NewLockView
      customResourceKindOpts={resourceKindOpts}
      simpleListOpts={{
        getFetchFn,
        getTable: getTableForSimpleList,
      }}
      hybridListOpts={{
        getFetchFn,
        getTable: getTableForHybridList,
      }}
    />
  );
}

export const resourceKindOpts: LockResourceOption[] = [
  ...baseResourceKindOpts,
  {
    value: 'access_request',
    label: 'Access Requests',
    listKind: 'simple',
  },
  {
    value: 'device',
    label: 'Trusted Devices',
    listKind: 'hybrid',
  },
];
