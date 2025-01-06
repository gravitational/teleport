/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { ReactNode } from 'react';
import styled from 'styled-components';

import { Box } from 'design';

type MultiRowBoxProps = {
  children: ReactNode;
};

/**
 * A box that displays a number of rows inside a rounded border, with horizontal
 * lines between rows. Use together with {@link Row}. Example:
 *
 * ```tsx
 * <MultiRowBox>
 *   <Row>Row 1</Row>
 *   <Row>Row 2</Row>
 * </MultiRowBox>
 * ```
 */
export const MultiRowBox = styled(Box)`
  border: ${props =>
    `${props.theme.borders[1]} ${props.theme.colors.interactive.tonal.neutral[2]}`};
  border-radius: ${props => props.theme.radii[2]}px;
`;

/** A single row of a {@link MultiRowBox}. */
export const Row = styled(Box)`
  padding: ${props => props.theme.space[4]}px;
  &:not(:last-child) {
    border-bottom: ${props =>
      `${props.theme.borders[1]} ${props.theme.colors.interactive.tonal.neutral[2]}`};
  }
`;

/**
 * A convenience utility to quickly render some components inside a single row
 * with a rounded border.
 */
export function SingleRowBox({ children }: MultiRowBoxProps) {
  return (
    <MultiRowBox>
      <Row>{children}</Row>
    </MultiRowBox>
  );
}
