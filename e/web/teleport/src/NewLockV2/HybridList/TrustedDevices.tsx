import Table from 'design/DataTable';
import {
  renderActionCell,
  HybridListProps,
} from 'teleport/LocksV2/NewLock/ResourceList/common';

import { TrustedDevice } from 'teleport/DeviceTrust/types';

import { IconCell } from 'e-teleport/DeviceTrust/DeviceList/DeviceList';

export function TrustedDevices(
  props: HybridListProps & { devices: TrustedDevice[] }
) {
  const {
    fetchNextPage,
    selectedResources,
    toggleSelectResource,
    fetchStatus,
    devices,
    pageSize,
  } = props;

  return (
    <Table
      data={devices}
      columns={[
        {
          key: 'osType',
          headerText: 'OS Type',
          render: ({ osType }) => <IconCell osType={osType} />,
        },
        {
          key: 'assetTag',
          headerText: 'Asset Tag',
        },
        {
          key: 'enrollStatus',
          headerText: 'Enroll Status',
        },
        {
          key: 'owner',
          headerText: 'Owner',
        },
        {
          altKey: 'action-btn',
          render: ({ id }) =>
            renderActionCell(Boolean(selectedResources.device[id]), () =>
              toggleSelectResource({ kind: 'device', targetValue: id })
            ),
        },
      ]}
      emptyText="No Devices Found"
      pagination={{ pageSize }}
      fetching={{
        onFetchMore: fetchNextPage,
        fetchStatus: fetchStatus,
      }}
      isSearchable
    />
  );
}
