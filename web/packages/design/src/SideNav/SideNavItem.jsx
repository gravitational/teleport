/*
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

import { borderColor } from './../system';
import Flex from './../Flex';

const fromTheme = ({ theme }) => {
  return {
    paddingLeft: `${theme.space[9]}px`,
    paddingRight: `${theme.space[5]}px`,
    background: theme.colors.levels.surface,
    color: theme.colors.text.slightlyMuted,
    fontSize: theme.fontSizes[1],
    fontWeight: theme.bold,
    '&:active, &.active': {
      borderLeftColor: theme.colors.brand,
      background: theme.colors.levels.elevated,
      color: theme.colors.text.main,
    },
    '&:hover, &:focus': {
      background: theme.colors.levels.elevated,
      color: theme.colors.text.main,
    },
  };
};

const SideNavItem = styled(Flex)`
  min-height: 56px;
  align-items: center;
  justify-content: flex-start;
  border-left: 4px solid transparent;
  cursor: pointer;
  outline: none;
  text-decoration: none;
  width: 100%;
  ${fromTheme}
  ${borderColor}
`;

SideNavItem.displayName = 'SideNavItem';

export default SideNavItem;
