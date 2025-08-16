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

import { NavLink } from 'react-router-dom';
import styled from 'styled-components';

import { space, SpaceProps } from 'design/system';

interface TabsContainerProps extends SpaceProps {
  /**
   * It's an underlying background border bottom that
   * renders underneath the currently active tab border
   * bottom.
   */
  withBottomBorder?: boolean;
}

export const TabsContainer = styled.div<TabsContainerProps>`
  position: relative;
  display: flex;
  gap: ${p => p.theme.space[5]}px;
  align-items: center;
  border-bottom: ${p =>
    p.withBottomBorder ? `1px solid ${p.theme.colors.spotBackground[0]}` : 0};

  ${space}
`;

export const TabContainer = styled.div<{ selected?: boolean }>`
  padding: ${p => p.theme.space[1] + p.theme.space[2]}px
    ${p => p.theme.space[2]}px;
  position: relative;
  cursor: pointer;
  z-index: 2;
  opacity: ${p => (p.selected ? 1 : 0.5)};
  transition: opacity 0.3s linear;
  color: ${p => p.theme.colors.text.main};
  font-weight: 300;
  font-size: 22px;
  line-height: ${p => p.theme.space[5]}px;
  white-space: nowrap;
  text-decoration: none;

  &:hover {
    opacity: 1;
  }
`;

export const TabContainerNavLink = styled(TabContainer).attrs({
  as: NavLink,
})``;

export const TabBorder = styled.div`
  position: absolute;
  bottom: -1px;
  background: ${p => p.theme.colors.brand};
  height: 2px;
  transition: all 0.3s cubic-bezier(0.19, 1, 0.22, 1);
`;
