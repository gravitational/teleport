import React from 'react';
import Table, { Cell } from 'design/DataTable';

import * as Icons from 'design/Icon';

import { TrustedDeviceResponse } from 'e-teleport/services/devices/types';

export const DeviceList = ({
  items = [],
  pageSize = 50,
  fetchStatus = '',
  fetchData,
}: Props) => {
  return (
    <Table
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
      ]}
      emptyText="No Devices Found"
      pagination={{ pageSize }}
      fetching={{ onFetchMore: fetchData, fetchStatus }}
      isSearchable
    />
  );
};

export const IconCell = ({
  osType,
}: {
  osType: 'Windows' | 'Linux' | 'macOS';
}) => {
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

type Props = {
  items: TrustedDeviceResponse['items'];
  pageSize?: number;
  fetchStatus?: 'loading' | 'disabled' | '';
  fetchData?: () => void;
};
