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

import styled, { css } from 'styled-components';

export enum NavigationItemSize {
  Small,
  Large,
  Indented,
}

export const Icon = styled.div`
  flex: 0 0 24px;
  margin-right: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  svg {
    height: 24px;
    width: 24px;
  }
`;

export const SmallIcon = styled(Icon)`
  flex: 0 0 14px;
  margin-right: 10px;
  svg {
    height: 18px;
    width: 18px;
  }
`;

interface LinkContentProps {
  size: NavigationItemSize;
}

const padding = {
  [NavigationItemSize.Small]: '7px 0px 7px 30px',
  [NavigationItemSize.Indented]: '7px 30px 7px 67px',
  [NavigationItemSize.Large]: '16px 30px',
};

export const LinkContent = styled.div<LinkContentProps>`
  display: flex;
  padding: ${p => padding[p.size]};
  align-items: center;
  width: 100%;
  opacity: 0.7;
  transition: opacity 0.15s ease-in;
`;

export const commonNavigationItemStyles = css`
  display: flex;
  position: relative;
  color: ${props => props.theme.colors.text.main};
  text-decoration: none;
  user-select: none;
  font-size: 14px;
  font-weight: 300;
  border-left: 4px solid transparent;
  transition: transform 0.3s cubic-bezier(0.19, 1, 0.22, 1);
  will-change: transform;

  &:hover {
    background: ${props => props.theme.colors.spotBackground[0]};
  }
`;
