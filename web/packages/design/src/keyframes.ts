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

import { keyframes } from 'styled-components';

/**
 * Full rotation of an element.
 *
 * @example
 * import { rotate360 } from 'design'
 *
 * const Spinner = styled.div`
 *   animation: ${rotate360} 1s linear infinite;
 * `
 */
export const rotate360 = keyframes`
  from { transform: rotate(0deg);   }
  to   { transform: rotate(360deg); }
`;

// The animation should start from 100% opacity so that a transition from non-blinking state to a
// blinking state isn't abrupt.
export const blink = keyframes`
    0% {
      opacity: 100%;
    }
    50% {
      opacity: 0;
    }
    100% {
      opacity: 100%;
    }
  `;

export const disappear = keyframes`
to { opacity: 0; }
`;
