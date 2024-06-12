/**
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

export const Key = styled.div`
  line-height: 1;
  background: ${p => p.theme.colors.spotBackground[1]};
  padding: 2px;
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
  border-radius: ${p => p.theme.space[1]}px;
  font-weight: 700;
  color: ${p => p.theme.colors.text.muted};
`;

export const KeyShortcut = styled.div`
  display: flex;
  align-items: center;
  gap: ${p => p.theme.space[1]}px;
  opacity: 0.5;
  font-size: 12px;
  pointer-events: none;
  user-select: none;
  transition: opacity 0.2s ease-in-out;
`;
