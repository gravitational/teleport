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

import { NavLink } from 'react-router-dom';
import styled, { css } from 'styled-components';

export interface OpenProps {
  open: boolean;
}

export const STARTING_TRANSITION_DELAY = 80;
export const INCREMENT_TRANSITION_DELAY = 20;

export const Dropdown = styled.div<OpenProps>`
  position: absolute;
  display: flex;
  flex-direction: column;
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px;
  background: ${({ theme }) => theme.colors.levels.surface};
  box-shadow: ${({ theme }) => theme.boxShadow[1]};
  border-radius: ${p => p.theme.radii[2]}px;
  width: 265px;
  right: 20px;
  z-index: 999;
  opacity: ${p => (p.open ? 1 : 0)};
  visibility: ${p => (p.open ? 'visible' : 'hidden')};
  transform-origin: top right;
  transition:
    opacity 0.2s ease,
    visibility 0.2s ease,
    transform 0.3s cubic-bezier(0.45, 0.6, 0.5, 1.25);
  transform: ${p => (p.open ? 'scale(1)' : 'scale(.8)')};

  top: ${p => p.theme.topBarHeight[0]}px;
  @media screen and (min-width: ${p => p.theme.breakpoints.small}px) {
    top: ${p => p.theme.topBarHeight[1]}px;
  }
`;

export const DropdownItem = styled.div<{
  open?: boolean;
  $transitionDelay: number;
}>`
  line-height: 1;
  font-size: ${p => p.theme.fontSizes[2]}px;
  color: ${props => props.theme.colors.text.main};
  cursor: pointer;
  border-radius: ${p => p.theme.radii[2]}px;
  margin-bottom: ${p => p.theme.space[1]}px;
  opacity: ${p => (p.open ? 1 : 0)};
  transition:
    transform 0.3s ease,
    opacity 0.7s ease;
  transform: translate3d(${p => (p.open ? 0 : '20px')}, 0, 0);
  transition-delay: ${p => p.$transitionDelay}ms;

  &:hover,
  &:focus-within {
    background: ${props => props.theme.colors.spotBackground[0]};
  }

  &:last-of-type {
    margin-bottom: 0;
  }
`;

export const commonDropdownItemStyles = css`
  align-items: center;
  display: flex;
  padding: ${p => p.theme.space[1] * 3}px;
  color: ${props => props.theme.colors.text.main};
  text-decoration: none;
  outline: none;

  svg {
    height: 18px;
    width: 18px;
  }
`;

export const DropdownItemButton = styled.div`
  ${commonDropdownItemStyles};
`;

export const DropdownItemLink = styled(NavLink)`
  ${commonDropdownItemStyles};
`;

export const DropdownItemIcon = styled.div`
  margin-right: ${p => p.theme.space[3]}px;
  line-height: 0;
`;

export const DropdownDivider = styled.div`
  height: 1px;
  background: ${props => props.theme.colors.spotBackground[1]};
  margin: ${props => props.theme.space[1]}px;
  margin-top: 0;
`;
