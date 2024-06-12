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

import styled, { keyframes } from 'styled-components';

const loading = keyframes`
  0% {
    opacity: 0;
  }
  50% {
    opacity: 1;
  }
  100% {
    opacity: 0;
  }
`;

export const Typing = styled.div`
  margin: 0 30px 0 30px;
`;

export const TypingContainer = styled.div`
  position: relative;
  padding: 10px;
  display: flex;
`;

export const TypingDot = styled.div`
  width: 6px;
  height: 6px;
  margin-right: 6px;
  background: ${p => p.theme.colors.spotBackground[2]};
  border-radius: 50%;
  opacity: 0;
  animation: ${loading} 1.5s ease-in-out infinite;
`;
