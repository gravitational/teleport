/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { Box } from 'design';

type Base = {
  isVisible: boolean;
  skipAnimation: boolean;
  panelWidth: number;
  zIndex: number;
};

type RightPanel = Base & {
  slideFrom: 'right';
  right: number | string;
};

type LeftPanel = Base & {
  slideFrom: 'left';
  left: number | string;
};

type Props = RightPanel | LeftPanel;

/**
 * Panel that slides from right or left underneath the web UI's
 * top bar navigation.
 */
export const SlidingSidePanel = styled(Box).attrs({
  px: '5px',
})<Props>`
  position: fixed;
  height: 100%;
  scrollbar-color: ${p => p.theme.colors.spotBackground[2]} transparent;
  width: ${p => p.panelWidth}px;
  background: ${p => p.theme.colors.levels.surface};
  z-index: ${p => p.zIndex};

  ${props =>
    props.slideFrom === 'left'
      ? `left: ${props.left};
       border-right: 1px solid ${props.theme.colors.spotBackground[1]};`
      : `right: ${props.right};
       border-left: 1px solid ${props.theme.colors.spotBackground[1]};`}

  ${props =>
    props.isVisible
      ? `
      ${props.skipAnimation ? '' : 'transition: transform .15s ease-out;'}
      transform: translateX(0);
      `
      : `
      ${props.skipAnimation ? '' : 'transition: transform .15s ease-in;'}
      transform: translateX(${props.slideFrom === 'left' ? '-100%' : '100%'});
      `}


  top: ${p => p.theme.topBarHeight[0]}px;
  padding-bottom: ${p => p.theme.topBarHeight[0] + p.theme.space[2]}px;
  @media screen and (min-width: ${p => p.theme.breakpoints.small}px) {
    top: ${p => p.theme.topBarHeight[1]}px;
    padding-bottom: ${p => p.theme.topBarHeight[1] + p.theme.space[2]}px;
  }
`;
