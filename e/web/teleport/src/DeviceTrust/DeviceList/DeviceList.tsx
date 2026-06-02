import { ComponentType } from 'react';
import styled from 'styled-components';

import Box from 'design/Box';
import Table, { Cell, TextCell } from 'design/DataTable';
import * as icons from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';
import { ResourceIcon, ResourceIconName } from 'design/ResourceIcon';
import { P2 } from 'design/Text';

import {
  DeviceListProps,
  deviceSource,
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
          key: 'source',
          headerText: 'Source',
          render: ({ source }) => <TextCell data={deviceSource(source)} />,
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
  let resourceIconName: ResourceIconName;
  let Icon: ComponentType<IconProps>;
  switch (osType) {
    case 'Windows':
      resourceIconName = 'microsoft';
      break;
    case 'Linux':
      resourceIconName = 'linux';
      break;
    case 'macOS':
      resourceIconName = 'apple';
      break;
    case 'iOS':
      Icon = icons.DeviceMobileCamera;
      break;
    case 'iPadOS':
      Icon = icons.Tablet;
      break;
    default:
      osType satisfies never;
  }
  return (
    <Cell align="left" style={{ display: 'flex', alignItems: 'center' }}>
      {resourceIconName ? (
        <ResourceIcon name={resourceIconName} width="14px" mr={3} />
      ) : Icon ? (
        <Icon size={14} mr={3} />
      ) : undefined}
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
