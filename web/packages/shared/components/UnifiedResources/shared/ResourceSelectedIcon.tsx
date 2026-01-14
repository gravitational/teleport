/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { CheckThick } from 'design/Icon';

const AccessCheckBackground = styled.div`
  border-radius: 999px;
  background-color: ${({ theme }) =>
    theme.colors.interactive.solid.success.default};
  line-height: 0;
  color: white;
  border: 2px solid
    ${({ theme }) => theme.colors.interactive.solid.success.default};
`;

export const ResourceSelectedIcon = () => (
  <AccessCheckBackground>
    <CheckThick size="small" />
  </AccessCheckBackground>
);
