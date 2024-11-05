import React from 'react';
import Table, { Cell } from 'design/DataTable';

import * as Icons from 'design/Icon';

import {
  DeviceListProps,
  TrustedDeviceOSType,
} from 'teleport/DeviceTrust/types';

export const DeviceList = ({
  items = [],
  pageSize = 50,
  pagerPosition = null,
  fetchStatus = '',
  fetchData,
}: DeviceListProps) => {
  return (
    <Table
      css={`
        tbody tr {
          cursor: pointer;
          &:hover {
            background-color: ${p =>
              p.theme.colors.interactive.tonal.primary[2]};
          }
        }
      `}
      data={items}
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
      ]}
      emptyText="No Devices Found"
      pagination={{ pageSize, pagerPosition }}
      fetching={{ onFetchMore: fetchData, fetchStatus }}
      isSearchable
    />
  );
};

export const IconCell = ({ osType }: { osType: TrustedDeviceOSType }) => {
  let icon;
  switch (osType) {
    case 'Windows':
      icon = <Icons.Windows size="small" mr={1} />;
      break;
    case 'Linux':
      icon = <Icons.Linux size="small" mr={1} />;
      break;
    default:
      icon = <Icons.Apple size="small" mr={1} />;
  }
  return (
    <Cell align="left" style={{ display: 'flex' }}>
      {icon} {osType}
    </Cell>
  );
};
