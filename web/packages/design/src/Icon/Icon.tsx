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

import React, { PropsWithChildren } from 'react';
import styled from 'styled-components';

import { space, color, borderRadius } from 'design/system';

export function Icon({
  size = 'medium',
  children,
  ...otherProps
}: PropsWithChildren<Props>) {
  let iconSize = size;
  if (size === 'small') {
    iconSize = 16;
  }
  if (size === 'medium') {
    iconSize = 20;
  }
  if (size === 'large') {
    iconSize = 24;
  }
  if (size === 'extraLarge') {
    iconSize = 32;
  }
  return (
    <StyledIcon {...otherProps}>
      <svg
        fill="currentColor"
        height={iconSize}
        width={iconSize}
        viewBox="0 0 24 24"
      >
        {children}
      </svg>
    </StyledIcon>
  );
}

const StyledIcon = styled.span`
  display: inline-flex;
  align-items: center;
  justify-content: center;

  ${color};
  ${space};
  ${borderRadius};
`;

export type IconProps = {
  size?: 'small' | 'medium' | 'large' | 'extraLarge' | number;
  color?: string;
  title?: string;
  m?: number | string;
  mr?: number | string;
  ml?: number | string;
  mb?: number | string;
  mt?: number | string;
  my?: number | string;
  mx?: number | string;
  p?: number | string;
  pr?: number | string;
  pl?: number | string;
  pb?: number | string;
  pt?: number | string;
  py?: number | string;
  px?: number | string;
  role?: string;
  style?: React.CSSProperties;
  borderRadius?: number;
  onClick?: () => void;
  disabled?: boolean;
  as?: any;
  to?: string;
  className?: string;
};

type Props = IconProps & {
  children?: React.SVGProps<SVGPathElement> | React.SVGProps<SVGPathElement>[];
  a?: any;
};
