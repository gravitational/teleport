import { useEffect, useState } from 'react';

import { FetchStatus } from 'design/DataTable/types';
import useAttempt from 'shared/hooks/useAttemptNext';

import useTeleportE from 'e-teleport/useTeleportE';
import { TrustedDevice } from 'teleport/DeviceTrust/types';

// default api query limit size
const maxFetchLimit = 5000;

export const useDevices = () => {
  const ctx = useTeleportE();
  const requiredPermissions = [
    { value: ctx.storeUser.getDeviceTrustAccess().read, label: 'device.read' },
    {
      value: ctx.storeUser.getDeviceTrustAccess().list,
      label: 'device.list',
    },
  ];
  const missingPermissions = requiredPermissions
    .filter(perm => !perm.value)
    .map(perm => perm.label);
  const canList = missingPermissions.length === 0;

  const { attempt, setAttempt } = useAttempt(
    canList ? 'processing' : 'success'
  );

  // tableDataAndState holds device details and table state for DeviceList.tsx
  const [tableDataAndState, setTableDataAndState] = useState<tableState>({
    items: [],
    fetchStatus: 'disabled',
    startKey: '',
  });

  function fetchData() {
    // update fetchstatus
    setTableDataAndState({ ...tableDataAndState, fetchStatus: 'loading' });
    return ctx.deviceService
      .fetchDevices({
        limit: maxFetchLimit,
        startKey: tableDataAndState.startKey,
      })
      .then(response => {
        setAttempt({ status: 'success' });
        // update tableDataAndState with device list returned from api
        setTableDataAndState({
          items: [...tableDataAndState.items, ...response.items],
          fetchStatus: response?.startKey ? '' : 'disabled',
          startKey: response?.startKey,
        });
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
      });
  }

  useEffect(() => {
    if (canList) {
      fetchData();
    }
  }, [canList]);

  return {
    ...tableDataAndState,
    attempt,
    fetchData,
    missingPermissions,
  };
};

type tableState = {
  items: TrustedDevice[];
  fetchStatus: FetchStatus;
  startKey: string;
};
