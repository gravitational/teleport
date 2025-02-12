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

/**
 * Stack is a variant of Flex designed to distribute elements in a vertical space with consistent
 * spacing using the gap property. If no gap is specified, it defaults to 1.
 *
 * It's possible to "split the stack" by setting `margin-top: auto;` on a specific child. That child
 * and all children below it will be aligned to the bottom of the stack.
 *
 * Inspired by https://every-layout.dev/layouts/stack/. It follows the approach of styling the
 * context, not the individual elements, to achieve desired spacing.
 *
 * @example
 *
 * <Stack gap={3}>
 *   <Stack>
 *     <Breadcrumbs />
 *     <ComponentHeader/>
 *   </Stack>
 *
 *   <Stack gap={2}>
 *     <ComponentMainBody/>
 *     <ComponentSidenote />
 *   </Stack>
 * </Stack>
 */
export const Stack = styled(Flex).attrs({
  flexDirection: 'column',
})`
  // Prevents children from shrinking, within a stack we pretty much never want that to happen.
  // Individual children can override this.
  & > * {
    flex-shrink: 0;
  }
`;
Stack.defaultProps = {
  gap: 1,
  // align-items: flex-start lets children keep their original size. Otherwise elements like buttons
  // would occupy all available horizontal space instead of the minimal amount of space they need.
  //
  // This is set as a default prop, as in some cases it might be necessary to override align-items.
  alignItems: 'flex-start',
};
