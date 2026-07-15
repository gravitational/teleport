/*
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

import {
  ComponentPropsWithoutRef,
  PropsWithChildren,
  useCallback,
  useRef,
} from 'react';
import styled, { CSSProp } from 'styled-components';

import Popover, { PopoverProps } from '../Popover';
import getScrollbarSize from './../utils/scrollbarSize';

export default function Menu(
  props: PropsWithChildren<{
    onEntering?: (el: HTMLElement) => void;
    menuListCss?: CSSProp;
    menuListProps?: ComponentPropsWithoutRef<typeof MenuList>;
  }> &
    Pick<
      PopoverProps,
      | 'anchorEl'
      | 'onClose'
      | 'open'
      | 'popoverCss'
      | 'getContentAnchorEl'
      | 'anchorOrigin'
      | 'transformOrigin'
      | 'updatePositionOnChildResize'
    >
) {
  const {
    children,
    popoverCss,
    menuListCss,
    menuListProps,
    onEntering,
    anchorOrigin = { vertical: 'bottom', horizontal: 'right' },
    transformOrigin = { vertical: 'top', horizontal: 'right' },
    ...other
  } = props;

  const menuListRef = useRef<HTMLDivElement>(null);
  const defaultGetContentAnchorEl = useCallback(
    () => menuListRef.current?.firstChild as HTMLElement,
    []
  );

  const handleEntering = useCallback(
    (element: HTMLElement) => {
      if (!menuListRef.current) {
        return;
      }
      const menuList = menuListRef.current;

      // Let's ignore that piece of logic if users are already overriding the width
      // of the menu.
      if (
        menuList &&
        element.clientHeight < menuList.clientHeight &&
        !menuList.style.width
      ) {
        const size = `${getScrollbarSize()}px`;
        menuList.style['paddingRight'] = size;
        menuList.style.width = `calc(100% + ${size})`;
      }

      if (onEntering) {
        onEntering(element);
      }
    },
    [onEntering]
  );

  return (
    <Popover
      popoverCss={popoverCss}
      onEntering={handleEntering}
      anchorOrigin={anchorOrigin}
      transformOrigin={transformOrigin}
      // getContentAnchorEl can still be overriden if explicitly passed through props.
      getContentAnchorEl={defaultGetContentAnchorEl}
      {...other}
    >
      <MenuList {...menuListProps} css={menuListCss} ref={menuListRef}>
        {children}
      </MenuList>
    </Popover>
  );
}

export const MenuList = styled.div.attrs({ role: 'menu' })`
  background-color: ${props => props.theme.colors.levels.elevated};
  border-radius: 4px;
  box-shadow: ${props => props.theme.boxShadow[0]};
  box-sizing: border-box;
  max-height: calc(100% - 96px);
  overflow: hidden;
  overflow-y: auto;
  position: relative;
  padding: 0;
`;
