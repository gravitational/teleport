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

import { Box } from 'design';

type IconCircleProps = {
  Icon: React.ElementType;
  size: number;
  bg?: string;
  fill?: string;
};

/**
 * Returns an Icon with a circular background
 *
 * @remarks
 * background fill and text color will default to the current theme if not provided
 *
 * @param Icon - the Icon to render
 * @param bg - optional background color
 * @param size - the Icon size including the circle
 * @param fill - optional text color
 * @returns JSX Element
 *
 */
export const IconCircle = ({ Icon, bg, size, fill }: IconCircleProps) => {
  const theme = useTheme();
  const background = bg ? bg : theme.colors.spotBackground[0];
  const color = fill ? fill : theme.colors.text.main;

  return (
    <Box
      bg={background}
      borderRadius="50%"
      p={size / 4}
      width={size}
      height={size}
    >
      <Icon size={size / 2} fill={color} />
    </Box>
  );
};
