/*
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

import styled from 'styled-components';

import {
  alignItems,
  AlignItemsProps,
  flexBasis,
  FlexBasisProps,
  flexDirection,
  FlexDirectionProps,
  flexWrap,
  FlexWrapProps,
  gap,
  GapProps,
  justifyContent,
  JustifyContentProps,
} from 'design/system';

import Box, { BoxProps } from '../Box';

export interface FlexProps
  extends BoxProps,
    AlignItemsProps,
    JustifyContentProps,
    FlexWrapProps,
    FlexDirectionProps,
    FlexBasisProps,
    GapProps {
  /**
   * Uses inline-flex instead of just flex as the display property.
   */
  inline?: boolean;
}

const Flex = styled(Box)<FlexProps>`
  display: ${props => (props.inline ? 'inline-flex' : 'flex')};
  ${alignItems}
  ${justifyContent}
  ${flexWrap}
  ${flexBasis}
  ${flexDirection}
  ${gap};
`;

Flex.displayName = 'Flex';

export default Flex;
