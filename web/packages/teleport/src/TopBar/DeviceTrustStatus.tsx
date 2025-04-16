/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import styled from 'styled-components';

import { Flex, Text } from 'design';

import session from 'teleport/services/websession';

import { DeviceTrustIcon } from './DeviceTrustIcon';

export type DeviceTrustStatusKind = 'authorized' | 'warning' | 'none';

const DeviceTrustText = ({ kind }: { kind: DeviceTrustStatusKind }) => {
  if (kind === 'authorized') {
    return (
      <StatusText color="success.active">
        Session authorized with device trust.
      </StatusText>
    );
  }
  if (kind === 'warning') {
    return (
      <StatusText color="warning.active">
        Session is not authorized with Device Trust. Access is restricted.
      </StatusText>
    );
  }
  return null;
};

function getDeviceTrustStatusKind(
  deviceTrusted: boolean,
  deviceTrustRequired: boolean
): DeviceTrustStatusKind {
  if (deviceTrusted) {
    return 'authorized';
  }
  if (deviceTrustRequired) {
    return 'warning';
  }

  return 'none';
}

export const DeviceTrustStatus = ({
  iconOnly = false,
}: {
  iconOnly?: boolean;
}) => {
  const deviceTrustRequired = session.getDeviceTrustRequired();
  const deviceTrusted = session.getIsDeviceTrusted();
  const kind = getDeviceTrustStatusKind(deviceTrusted, deviceTrustRequired);
  if (kind === 'none') {
    return null;
  }

  return (
    <Wrapper $iconOnly={iconOnly}>
      <DeviceTrustIcon kind={kind} />
      {!iconOnly && <DeviceTrustText kind={kind} />}
    </Wrapper>
  );
};

const Wrapper = styled(Flex)<{ $iconOnly?: boolean }>`
  ${p => (p.$iconOnly ? 'padding-left: 16px' : 'padding: 12px;')}
  align-items: center;
`;

const StatusText = styled(Text)`
  font-size: ${p => p.theme.fontSizes[1]}px;
  margin-left: 16px;
  font-style: italic;
`;
