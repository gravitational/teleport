import React from 'react';
import styled from 'styled-components';
import Table, { Cell } from 'design/DataTable';

import {
  DeviceListProps,
  TrustedDeviceOSType,
} from 'teleport/DeviceTrust/types';
import { P2 } from 'design/Text';
import Box from 'design/Box';
import { ResourceIcon, ResourceIconName } from 'design/ResourceIcon';

export const DeviceList = ({
  items = [],
  pageSize = 50,
  pagerPosition = null,
  fetchStatus = '',
  fetchData,
}: DeviceListProps) => {
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
          render: ({ enrollStatus }) => (
            <EnrollmentStatusCell status={enrollStatus} />
          ),
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

const EnrollmentStatusCell = ({ status }: { status: string }) => {
  const enrolled = status === 'enrolled';
  return (
    <Cell
      align="left"
      css={`
        display: flex;
        align-items: center;
      `}
    >
      <EnrollmentIcon enrolled={enrolled} />
      <P2 color={enrolled ? 'success.main' : 'error.main'}>{status}</P2>
    </Cell>
  );
};

export const IconCell = ({ osType }: { osType: TrustedDeviceOSType }) => {
  let iconName: ResourceIconName;
  switch (osType) {
    case 'Windows':
      iconName = 'microsoft';
      break;
    case 'Linux':
      iconName = 'linux';
      break;
    case 'macOS':
      iconName = 'apple';
      break;
  }
  return (
    <Cell align="left" style={{ display: 'flex', alignItems: 'center' }}>
      <ResourceIcon name={iconName} width="14px" mr={3} />
      {osType}
    </Cell>
  );
};

const EnrollmentIcon = styled(Box)<{ enrolled: boolean }>`
  width: 12px;
  height: 12px;
  margin-right: ${p => p.theme.space[1]}px;
  border-radius: 50%;
background-color: ${p =>
  p.enrolled ? p.theme.colors.success.main : p.theme.colors.error.main};
  };
`;
