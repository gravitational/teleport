/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { Cross } from 'design/Icon';
import type { Attempt } from 'shared/hooks/useAttemptNext';

export const SecondaryCrossIcon = <T,>({
  clearAttempt,
  toggleResource,
  item,
  createAttempt,
}: {
  clearAttempt: () => void;
  toggleResource: (resource: T) => void;
  item: T;
  createAttempt: Attempt;
}) => (
  <Cross
    size="small"
    borderRadius={2}
    p={2}
    onClick={() => {
      clearAttempt();
      toggleResource(item);
    }}
    disabled={createAttempt.status === 'processing'}
    css={`
      cursor: pointer;
      background-color: transparent;

      &:hover,
      &:focus-visible {
        background-color: ${({ theme }) => theme.colors.spotBackground[0]};
      }
    `}
  />
);
