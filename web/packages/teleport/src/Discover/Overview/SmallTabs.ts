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

import styled from 'styled-components';

import { TabContainer, TabsContainer } from 'design/Tabs';

export const SmallTabsContainer = styled(TabsContainer)`
  gap: ${p => p.theme.space[3]}px;
`;

export const SmallTab = styled(TabContainer)`
  font-size: 14px;
  line-height: 20px;
  font-weight: 500;
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[2]}px;
`;
