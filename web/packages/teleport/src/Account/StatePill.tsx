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
type State = 'active' | 'inactive';

export interface StatePillProps {
  state: State;
}

/**
 * Renders a pill component that represents a state of an authentication method.
 * The `state` property is both an enum value, as well as the UI text.
 */
export function StatePill({ state }: StatePillProps) {
  return <StatePillBody state={state}>{state}</StatePillBody>;
}

const StatePillBody = styled.span<StatePillProps>`
  font-size: 14px;
  display: inline-block;
  margin: 0 ${props => props.theme.space[2]}px;
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
  }
}
