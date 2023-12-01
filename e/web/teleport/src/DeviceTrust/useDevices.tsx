import { useEffect, useState } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import { FetchStatus } from 'design/DataTable/types';

import { TrustedDevice } from 'teleport/DeviceTrust/types';

import useTeleportE from 'e-teleport/useTeleportE';

// default api query limit size
const maxFetchLimit = 5000;

export const useDevices = () => {
  const ctx = useTeleportE();

  const { attempt, setAttempt } = useAttempt('processing');

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
    fetchData();
  }, []);

  return {
    ...tableDataAndState,
    attempt,
    fetchData,
    showTrustedDevicesCTA: ctx.lockedFeatures.trustedDevices,
  };
};

type tableState = {
  items: TrustedDevice[];
  fetchStatus: FetchStatus;
  startKey: string;
};

export type State = ReturnType<typeof useDevices>;
