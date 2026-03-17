/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { useTheme } from 'styled-components';

import { Dot } from 'design/Icon';
import type { IconProps } from 'design/Icon/Icon';
import type { Theme } from 'design/theme';

import type { StatusKind } from './Status';
import { getKindColors } from './statusColors';

export interface StatusDotProps extends IconProps {
  kind?: StatusKind;
}

// A small colored dot to indicate status kind.
export function StatusDot({
  kind = 'neutral',
  size = 'small',
  ...rest
}: StatusDotProps) {
  const theme = useTheme() as Theme;
  const color = getKindColors(theme, kind).solid;

  return <Dot size={size} color={color} {...rest} />;
}
