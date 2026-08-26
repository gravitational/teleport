/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import React, { PropsWithChildren } from 'react';
import styled from 'styled-components';

import Flex from 'design/Flex';
import { IconProps } from 'design/Icon/Icon';

import Label, { LabelProps } from './Label';

export function LabelButtonWithIcon({
  IconLeft,
  IconRight,
  children,
  title,
  withHoverState = false,
  ...labelProps
}: {
  IconLeft?: React.ComponentType<IconProps>;
  IconRight?: React.ComponentType<IconProps>;
  onClick?: () => void;
  title?: string;
  withHoverState?: boolean;
} & LabelProps &
  PropsWithChildren) {
  const Icon = IconLeft ?? IconRight;

  let icon;
  if (Icon) {
    icon = (
      <ButtonIcon>
        <Icon size="small" />
      </ButtonIcon>
    );
  }

  return (
    <LabelWithHoverAffect
      {...labelProps}
      title={title}
      withHoverState={withHoverState}
      tabIndex={0}
    >
      <Flex gap={1} alignItems="center">
        {IconLeft && icon}
        {children}
        {IconRight && icon}
      </Flex>
    </LabelWithHoverAffect>
  );
}

const ButtonIcon = styled.div`
  align-items: center;
  display: flex;
`;

const LabelWithHoverAffect = styled(Label)`
  &:hover {
    cursor: ${p => (p.withHoverState ? 'pointer' : 'default')};
  }
`;
