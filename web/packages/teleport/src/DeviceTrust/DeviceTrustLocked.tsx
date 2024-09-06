/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React from 'react';
import styled from 'styled-components';

import { Box, Flex, Link } from 'design';

import { IconCircle } from 'design/Icon/IconCircle';

import { Windows, Linux, Apple } from 'design/Icon';

import { LockIcon } from 'design/SVGIcon';

import Table, { Cell } from 'design/DataTable';

import { P } from 'design/Text/Text';

import {
  DeviceListProps,
  TrustedDeviceOSType,
} from 'teleport/DeviceTrust/types';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { CtaEvent } from 'teleport/services/userEvent';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';

export function DeviceTrustLocked() {
  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Trusted Devices</FeatureHeaderTitle>
      </FeatureHeader>
      <Box
        mr="6"
        mb="4"
        style={{
          filter: 'blur(2px)',
          pointerEvents: 'none',
          userSelect: 'none',
          height: '100px',
        }}
      >
        <FakeDeviceList
          items={generateFakeItems(15)}
          fetchData={() => null}
          fetchStatus={''}
        />
      </Box>
      <StyledMessageContainer>
        <Box p="3" borderRadius="50%">
          <IconCircle Icon={LockIcon} size={64} />
        </Box>
        <P textAlign="justify">
          Device Trust enables trusted and authenticated device access. When
          resources are configured with the Device Trust mode “required”,
          Teleport will authenticate the Trusted Device, in addition to
          establishing the user's identity and enforcing the necessary roles,
          and leaves an audit trail with device information. For more details,
          please view{' '}
          <Link
            href={
              'https://goteleport.com/docs/access-controls/device-trust/guide/'
            }
            target="_blank"
          >
            Device Trust documentation
          </Link>
          .
        </P>
        <Box>
          <ButtonLockedFeature event={CtaEvent.CTA_TRUSTED_DEVICES}>
            Unlock Device Trust with Teleport Enterprise
          </ButtonLockedFeature>
        </Box>
      </StyledMessageContainer>
    </FeatureBox>
  );
}

function generateFakeItems(count) {
  const items = [];
  const osType = ['Windows', 'Linux', 'macOS'];

  for (let i = 0; i < count; i++) {
    items.push({
      id: `id-${i}`,
      assetTag: `CLFBDAACC${i}`,
      enrollStatus: `enroll-status-${i}`,
      osType: osType[i % osType.length],
    });
  }

  return items;
}

const FakeDeviceList = ({
  items = [],
  pageSize = 20,
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
        },
      ]}
      emptyText="No Devices Found"
      pagination={{ pageSize }}
      fetching={{ onFetchMore: fetchData, fetchStatus }}
    />
  );
};

const IconCell = ({ osType }: { osType: TrustedDeviceOSType }) => {
  let icon;
  switch (osType) {
    case 'Windows':
      icon = <Windows size="small" mr={1} />;
      break;
    case 'Linux':
      icon = <Linux size="small" mr={1} />;
      break;
    default:
      icon = <Apple size="small" mr={1} />;
  }
  return (
    <Cell align="left" style={{ display: 'flex' }}>
      {icon} {osType}
    </Cell>
  );
};

const StyledMessageContainer = styled(Flex)`
  position: relative;
  top: 30%;
  left: 50%;
  transform: translate(-50%, -50%);
  background-color: ${({ theme }) => theme.colors.levels.elevated};
  flex-direction: column;
  justify-content: center;
  align-items: center;
  padding: 24px;
  gap: 24px;
  width: 600px;
  box-shadow:
    0 5px 5px -3px rgba(0, 0, 0, 0.2),
    0 8px 10px 1px rgba(0, 0, 0, 0.14),
    0 3px 14px 2px rgba(0, 0, 0, 0.12);
  border-radius: 8px;
`;
