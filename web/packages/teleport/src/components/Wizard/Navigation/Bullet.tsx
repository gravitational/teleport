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

import type { JSX } from 'react';
import styled from 'styled-components';

import Flex from 'design/Flex';

export type Props = {
  isDone?: boolean;
  isActive?: boolean;
  stepNumber?: number;
  Icon?: JSX.Element;
};

export function Bullet({ isDone, isActive, stepNumber, Icon }: Props) {
  if (Icon) {
    return <Flex mr={2}>{Icon}</Flex>;
  }

  if (isActive) {
    return <ActiveBullet data-testid="bullet-active" />;
  }

  if (isDone) {
    return <CheckedBullet data-testid="bullet-checked" />;
  }

  return (
    <BulletContainer data-testid="bullet-default">{stepNumber}</BulletContainer>
  );
}

export const BulletContainer = styled.span`
  height: 14px;
  width: 14px;
  border: 1px solid ${p => p.theme.colors.text.disabled};
  font-size: ${p => p.theme.fontSizes[0]}px;
  border-radius: 50%;
  margin-right: ${p => p.theme.space[2]}px;
  display: flex;
  align-items: center;
  justify-content: center;
`;

export const ActiveBullet = styled(BulletContainer)`
  border-color: ${props => props.theme.colors.brand};
  background: ${props => props.theme.colors.brand};

  &:before {
    content: '';
    height: 8px;
    width: 8px;
    border-radius: 50%;
    border: ${p => p.theme.radii[1]}px solid
      ${p => p.theme.colors.levels.surface};
  }
`;

export const CheckedBullet = styled(BulletContainer)`
  border-color: ${props => props.theme.colors.brand};
  background: ${props => props.theme.colors.brand};

  &:before {
    content: '✓';
    color: ${props => props.theme.colors.levels.popout};
  }
`;
