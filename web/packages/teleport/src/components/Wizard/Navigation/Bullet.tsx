/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import Flex from 'design/Flex';
import React from 'react';
import styled from 'styled-components';

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
  font-size: ${p => p.theme.fontSizes[1]}px;
  border-radius: 50%;
  margin-right: ${p => p.theme.space[2]}px;
  display: flex;
  align-items: center;
  justify-content: center;
`;

export const ActiveBullet = styled(BulletContainer)`
  border-color: ${props => props.theme.colors.brand};
  background: ${props => props.theme.colors.brand};

  :before {
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

  :before {
    content: '✓';
    color: ${props => props.theme.colors.levels.popout};
  }
`;
