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

export const StepTitle = styled.div`
  display: flex;
  align-items: center;
`;

export const StepsContainer = styled.div<{ active?: boolean }>`
  display: flex;
  flex-direction: column;
  color: ${p => (p.active ? 'inherit' : p.theme.colors.text.slightlyMuted)};
  margin-right: ${p => p.theme.space[5]}px;
  position: relative;

  &:after {
    position: absolute;
    content: '';
    width: 16px;
    background: ${({ theme }) => theme.colors.brand};
    height: 1px;
    top: 50%;
    transform: translate(0, -50%);
    right: -25px;
  }

  &:last-of-type {
    &:after {
      display: none;
    }
  }
`;
