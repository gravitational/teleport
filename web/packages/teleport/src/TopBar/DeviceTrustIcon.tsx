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

import { ReactNode } from 'react';
import styled from 'styled-components';

import { Flex } from 'design';
import { ShieldCheck, ShieldWarning } from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';

import { DeviceTrustStatusKind } from './DeviceTrustStatus';

export const DeviceTrustIcon = ({ kind }: { kind: DeviceTrustStatusKind }) => {
  const iconSize = 18;

  if (kind === 'authorized') {
    return (
      <ShieldIcon
        Icon={ShieldCheck}
        iconSize={iconSize}
        color="success.active"
        data-testid="device-trusted-icon"
      />
    );
  }

  return (
    <ShieldIcon
      Icon={ShieldWarning}
      iconSize={iconSize}
      color="warning.active"
      data-testid="device-trust-required-icon"
    />
  );
};

const ShieldIcon = ({
  Icon,
  iconSize,
  color,
  ...props
}: {
  Icon: (props: IconProps) => ReactNode;
  iconSize: number;
  color: string;
}) => {
  return (
    <Wrapper {...props}>
      <Icon color={color} size={iconSize} />
    </Wrapper>
  );
};

const Wrapper = styled(Flex)`
  height: 100%;
`;
