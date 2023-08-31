/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
      <Icon size={size / 2} bg={background} fill={color} />
    </Box>
  );
};
