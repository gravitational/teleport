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

import {
  ComponentPropsWithoutRef,
  ReactElement,
  useRef,
  useState,
} from 'react';

import { ButtonBorder, Flex, Menu, MenuItem } from 'design';
import { ButtonSize } from 'design/Button';
import * as icons from 'design/Icon';
import {
  LoginItem,
  MenuInputType,
  MenuLogin,
} from 'shared/components/MenuLogin';

/**
 * Displays a menu button with additional action menu attached to the right of it.
 * <MenuLoginWithActionMenu> expects two children, a LoginItem for <MenuLogin> and <MenuItem>
 * for action memnu.
 *
 * @example
 * <MenuLoginWithActionMenu
 *   buttonText="Log In"
 *   size="medium"
 *   onSelect={() => alert('item selected')}
 *   getLoginItems= {() => LoginItem[]}
 * >
 *   <MenuItem>Foo</MenuItem>
 *   <MenuItem>Bar</MenuItem>
 * </MenuLoginWithActionMenu>
 */
export const MenuLoginWithActionMenu = ({
  buttonText,
  onSelect,
  getLoginItems,
  children,
  menuWidth,
  width,
  size = 'medium',
  placeholder,
  inputType,
}: {
  /** Button text for main menu button. */
  buttonText?: string;
  /**
   * Handles select or click in main menu items.
   * If isExternalUrl item returned by getLoginItems is true, a button with <a> tag is rendered
   * and the value of url is passed for the login param. Since <a> tag with href
   * attribute handles onClick by default, the caller may wish to
   * pass an empty onSelect function value.
   */
  onSelect: (e: React.SyntheticEvent, login: string) => void;
  /** Fetches login items. */
  getLoginItems: () => LoginItem[] | Promise<LoginItem[]>;
  /** Action menu items. */
  children: MenuItemComponent | MenuItemComponent[];
  /**
   * Width of just the MenuLogin part of the component. Ignored if width is set.
   */
  menuWidth?: ComponentPropsWithoutRef<typeof MenuLogin>['width'];
  /**
   * Width of the whole component (button of MenuLogin + ButtonBorder of action menu). menuWidth is ignored if width is set.
   */
  width?: ComponentPropsWithoutRef<typeof Flex>['width'];
  size?: ButtonSize;
  /** Text for action menu search box or static label.  */
  placeholder?: string;
  /** Input type for menu item filter input. */
  inputType?: MenuInputType;
}) => {
  const moreButtonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  return (
    <Flex width={width}>
      <MenuLogin
        width={width ? '100%' : menuWidth}
        inputType={inputType}
        onSelect={onSelect}
        textTransform="none"
        alignButtonWidthToMenu
        getLoginItems={getLoginItems}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'right',
        }}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'right',
        }}
        style={{ borderTopRightRadius: 0, borderBottomRightRadius: 0 }}
        buttonText={buttonText}
        placeholder={placeholder}
      />
      <ButtonBorder
        ref={moreButtonRef}
        px={1}
        size={size}
        onClick={() => setIsOpen(true)}
        /* css value matches with ButtonWithMenu component */
        css={`
          border-left: none;
          border-top-left-radius: 0;
          border-bottom-left-radius: 0;
        `}
        title="Open menu"
      >
        <icons.MoreVert size={size} color="text.slightlyMuted" />
      </ButtonBorder>
      <Menu
        anchorEl={moreButtonRef.current}
        open={isOpen}
        onClose={() => setIsOpen(false)}
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

// TODO(ravicious,sshah): At the moment, this doesn't accomplish much – it only prevents MenuLoginWithActionMenu
// from MenuLoginWithActionMenu strings as children. Once styled-components are typed, it should enforce that
// MenuLoginWithActionMenu accepts only MenuItem as children.
type MenuItemComponent = ReactElement<typeof MenuItem>;
