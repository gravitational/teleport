import React from 'react';
import styled, { useTheme } from 'styled-components';

import Box from 'design/Box';

import Text from 'design/Text';

import Flex from 'design/Flex';

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';

import { Lock } from 'design/Icon';

import Link from 'design/Link';

import {
  TrustedDevice,
  TrustedDeviceOSType,
} from 'e-teleport/services/devices/types';

import { DeviceList } from './DeviceList';
import { deviceTrustDocUrl } from './DeviceTrust';

export function DeviceTrustLocked() {
  const theme = useTheme();
  return (
    // render a blurred out fake device list on background
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
        <DeviceList
          items={generateFakeItems(15)}
          fetchData={() => null}
          fetchStatus={''}
        />
      </Box>
      <StyledMessageContainer>
        <Box bgColor={theme.colors.spotBackground[0]} p="3" borderRadius="50%">
          <Text fontSize="26px">
            <Lock />
          </Text>
        </Box>
        <Text fontSize="2" textAlign="center">
          Device Trust allows Teleport admins to enforce the use of trusted
          devices, establishes each user's identity, and enforces the necessary
          roles. Any user of a trusted device will leave audit trails that
          include the device's information. Resources protected by the device
          mode “required” will enforce authenticated device access. For more
          information on Device Trust, please{' '}
          <Link href={deviceTrustDocUrl} target="_blank">
            view our documentation
          </Link>
          .
        </Text>
        <Box width="400px">
          <ButtonLockedFeature>
            Unlock Device Trust with Teleport Enterprise
          </ButtonLockedFeature>
        </Box>
      </StyledMessageContainer>
    </Box>
  );
}

const StyledMessageContainer = styled(Flex)`
  flex-direction: row;
  position: absolute;
  top: 50%;
  left: 50%;
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

function generateFakeItems(count: number): TrustedDevice[] {
  const items: TrustedDevice[] = [];
  const osType: TrustedDeviceOSType[] = ['Windows', 'Linux', 'macOS'];

  for (let i = 0; i < count; i++) {
    items.push({
      id: `id-${i}`,
      assetTag: `asset-tag-${i}`,
      enrollStatus: `enroll-status-${i}`,
      osType: osType[i % osType.length],
    });
  }

  return items;
}
