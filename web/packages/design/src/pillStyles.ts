/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { css } from 'styled-components';

// Shared base styles for pill-shaped inline elements (Status, Tag).
// Provides consistent sizing, shape, and overflow behavior.
export const pillBase = css`
  display: inline-flex;
  align-items: center;
  gap: ${p => p.theme.space[1]}px;
  padding: 2px ${p => p.theme.space[2]}px;
  border-radius: 98px;
  ${p => p.theme.typography.body3}
  white-space: nowrap;
  box-sizing: border-box;
  overflow: hidden;
  max-width: 100%;
  min-width: 0;
  vertical-align: middle;
`;
