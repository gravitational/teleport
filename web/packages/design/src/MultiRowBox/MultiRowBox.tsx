import React, { ReactNode } from 'react';

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
