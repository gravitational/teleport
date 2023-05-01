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
      icon = <Icons.Windows style={{ paddingRight: 5 }} />;
      break;
    case 'Linux':
      icon = <Icons.Linux style={{ paddingRight: 5 }} />;
      break;
    default:
      icon = <Icons.Apple style={{ paddingRight: 5 }} />;
  }
  return (
    <Cell align="left">
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
