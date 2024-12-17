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

import { TrustedDevice } from 'teleport/DeviceTrust/types';

import useTeleportE from 'e-teleport/useTeleportE';
import { AccessRequest } from 'e-teleport/services/workflow';

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
