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

import React from 'react';
import styled, { css } from 'styled-components';

/** State of an authentication method (password, MFA method, or passkey). */
export type AuthMethodState = 'active' | 'inactive';

interface StatePillProps {
  state: AuthMethodState | undefined;
  'data-testid'?: string;
}

/**
 * Renders a pill component that represents a state of an authentication method.
 * The `state` property is both an enum value, as well as the UI text.
 */
export function StatePill({ state, 'data-testid': testId }: StatePillProps) {
  // Explicitly return an empty element to ensure that potential future changes
  // to the pill body style won't result as an ugly element with no text. At the
  // same time, retain the test ID to simplify testing.
  if (!state) return <span data-testid={testId}></span>;
  return (
    <StatePillBody state={state} data-testid={testId}>
      {state}
    </StatePillBody>
  );
}

const StatePillBody = styled.span<StatePillProps>`
  font-size: 14px;
  display: inline-block;
  padding: 0 ${props => props.theme.space[3]}px;
  border-radius: 1000px;

  ${statePillStyles}
`;

function statePillStyles({ state }: StatePillProps): string {
  switch (state) {
    case 'active':
      return css`
        background-color: ${props =>
          props.theme.colors.interactive.tonal.success[0]};
        color: ${props => props.theme.colors.success.main};
      `;
    case 'inactive':
      return css`
        background-color: ${props =>
          props.theme.colors.interactive.tonal.neutral[0]};
        color: ${props => props.theme.colors.text.disabled};
      `;
    default:
      state satisfies never;
  }
}
