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

import { Box, Flex } from 'design';
import { typography } from 'design/system';

const Tabs = props => {
  return (
    <StyledTabs height="40px" color="text.slightlyMuted" as="nav" {...props} />
  );
};

export const TabItem = ({ title }) => <StyledTabItem>{title}</StyledTabItem>;

const StyledTabItem = styled(Box)`
  max-width: 200px;
  height: 100%;
  outline: none;
  text-transform: uppercase;
  text-decoration: none;
  color: inherit;
  align-items: center;
  display: flex;
  font-size: 11px;
  justify-content: center;
  flex: 1;

  &:hover,
  &.active,
  &:focus {
    color: ${props => props.theme.colors.text.main};
  }

  ${({ theme }) => ({
    backgroundColor: theme.colors.levels.sunken,
    color: theme.colors.text.main,
    fontWeight: 'bold',
    transition: 'none',
  })}

  ${({ theme }) => {
    return {
      border: 'none',
      borderRight: `1px solid ${theme.colors.levels.sunken}`,
      '&:hover, &:focus': {
        color: theme.colors.text.main,
        transition: 'color .3s',
      },
    };
  }}
`;

const StyledTabs = styled(Flex)`
  ${typography}
`;

export default Tabs;
