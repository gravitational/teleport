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
import { useTheme } from 'styled-components';

export function SVGIcon({
  children,
  viewBox = '0 0 20 20',
  size = 20,
  height,
  width,
  fill,
  ...svgProps
}: Props & React.SVGProps<SVGSVGElement>) {
  const theme = useTheme();

  return (
    <svg
      data-testid="svg"
      viewBox={viewBox}
      xmlns="http://www.w3.org/2000/svg"
      width={width || size}
      height={height || size}
      fill={fill || theme.colors.text.main}
      {...svgProps}
    >
      {children}
    </svg>
  );
}

interface Props {
  children: React.SVGProps<SVGPathElement> | React.SVGProps<SVGPathElement>[];
  fill?: string;
  size?: number;
  height?: number;
  width?: number;
  viewBox?: string;
}
