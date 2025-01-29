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

import {
  ComponentPropsWithoutRef,
  ComponentType,
  ElementType,
  ReactElement,
  useRef,
  useState,
} from 'react';

import { ButtonBorder, Flex, Menu, MenuItem } from 'design';
import { ButtonSize } from 'design/Button';
import * as icons from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';

/**
 * Displays a button with a menu to the right of it. Unlike with a regular <Button>, the text of
 * the button is set through the text prop. <ButtonWithMenu> expects its children to be <MenuItem>
 * components.
 *
 * The icon of the menu can be modified by passing MenuIcon. Props other than text and MenuIcon are
 * passed to the main <ButtonBorder>. size is automatically passed to both <ButtonBorder> (one for
 * the main button, one for the menu button) and the icon.
 *
 * Clicking on a menu item automatically closes the menu. This is done through event propagation.
 * Individual menu items can prevent the menu from closing by calling `event.stopPropagation` on the
 * onClick event.
 *
 * @example
 * <ButtonWithMenu
 *   text="Text in the button"
 *   size="large"
 *   MenuIcon={icons.Cog}
 * >
 *   <MenuItem>Foo</MenuItem>
 *   <MenuItem>Bar</MenuItem>
 * </ButtonWithMenu>
 */
export const ButtonWithMenu = <Element extends ElementType = 'button'>(
  props: {
    text: string;
    children: MenuItemComponent | MenuItemComponent[];
    MenuIcon?: ComponentType<IconProps>;
    size?: ButtonSize;
    forwardedAs?: Element;
  } & ComponentPropsWithoutRef<typeof ButtonBorder<Element>>
) => {
  const {
    text,
    MenuIcon = icons.MoreVert,
    children,
    size = 'medium',
    ...buttonBorderProps
  } = props;

  const moreButtonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);

  return (
    <Flex>
      <ButtonBorder
        css={`
          border-top-right-radius: 0;
          border-bottom-right-radius: 0;
        `}
        size={size}
        {...buttonBorderProps}
      >
        {text}
      </ButtonBorder>
      <ButtonBorder
        setRef={moreButtonRef}
        px={1}
        size={size}
        onClick={() => setIsOpen(true)}
        css={`
          border-left: none;
          border-top-left-radius: 0;
          border-bottom-left-radius: 0;
        `}
        title="Open menu"
      >
        <MenuIcon size={size} color="text.slightlyMuted" />
      </ButtonBorder>
      <Menu
        anchorEl={moreButtonRef.current}
        open={isOpen}
        onClose={() => setIsOpen(false)}
        // hack to properly position the menu
        getContentAnchorEl={null}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'right',
        }}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'right',
        }}
        menuListProps={{ onClick: () => setIsOpen(false) }}
      >
        {children}
      </Menu>
    </Flex>
  );
};

// TODO(ravicious): At the moment, this doesn't accomplish much – it only prevents ButtonWithMenu
// from accepting strings as children. Once styled-components are typed, it should enforce that
// ButtonWithMenu accepts only MenuItem as children.
type MenuItemComponent = ReactElement<typeof MenuItem>;
