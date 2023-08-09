/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import styled, { useTheme } from 'styled-components';

import Box from 'design/Box';

import Text from 'design/Text';

import Flex from 'design/Flex';

import Link from 'design/Link';

import { IconCircle } from 'design/Icon/IconCircle';

import { Windows, Linux, Apple } from 'design/Icon';

import { LockIcon } from 'design/SVGIcon';

import Table, { Cell } from 'design/DataTable';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { CtaEvent } from 'teleport/services/userEvent';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';

export function DeviceTrustLocked() {
  const theme = useTheme();

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
  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Trusted Devices</FeatureHeaderTitle>
      </FeatureHeader>
      <Box position="relative">
        <Box
          width="100%"
          mr="6"
          mb="4"
          style={{
            filter: 'blur(2px)',
            pointerEvents: 'none',
            userSelect: 'none',
          }}
        >
          <FakeDeviceList
            items={generateFakeItems(15)}
            fetchData={() => null}
            fetchStatus={''}
          />
        </Box>
        <StyledMessageContainer>
          <Box
            bgColor={theme.colors.spotBackground[0]}
            p="3"
            borderRadius="50%"
          >
            <IconCircle Icon={LockIcon} size={64} />
          </Box>
          <Text fontSize="2" textAlign="center">
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
          </Text>
          <Box width="400px">
            <ButtonLockedFeature event={CtaEvent.CTA_TRUSTED_DEVICES}>
              Unlock Device Trust with Teleport Enterprise
            </ButtonLockedFeature>
          </Box>
        </StyledMessageContainer>
      </Box>
    </FeatureBox>
  );
}

const FakeDeviceList = ({
  items = [],
  pageSize = 20,
  fetchStatus = '',
  fetchData,
}: any) => {
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

const IconCell = ({ osType }) => {
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
  flex-direction: row;
  position: absolute;
  top: 50%;
  left: 55%;
  transform: translate(-50%, -50%);
  background-color: ${({ theme }) => theme.colors.levels.elevated};
  flex-direction: column;
  justify-content: center;
  align-items: center;
  padding: 24px;
  gap: 24px;
  width: 650px;
  box-shadow: 0 5px 5px -3px rgba(0, 0, 0, 0.2),
    0 8px 10px 1px rgba(0, 0, 0, 0.14), 0 3px 14px 2px rgba(0, 0, 0, 0.12);
  border-radius: 8px;
`;
