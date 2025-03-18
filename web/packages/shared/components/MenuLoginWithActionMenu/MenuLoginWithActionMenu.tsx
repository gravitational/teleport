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

import { ReactElement, useRef, useState } from 'react';

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
 * @param buttonText - button text for main menu button.
 * @param onSelect - handles select or click in main menu items.
 * @param getLoginItems - fetches login items.
 * @param children - action menu items.
 * @param placeholder - text for action menu search box or static label.
 * @param size - button icon size.
 * @param width - button width.
 * @param disableSearchAndFilter - instructs underlying MenuLogin component to display static
 *                                 label text instead of default search box.
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
  width,
  size = 'medium',
  placeholder,
  disableSearchAndFilter,
}: {
  buttonText: string;
  onSelect: (e: React.SyntheticEvent, login: string) => void;
  getLoginItems: () => LoginItem[] | Promise<LoginItem[]>;
  children: MenuItemComponent | MenuItemComponent[];
  width?: string;
  size?: ButtonSize;
  placeholder?: string;
  disableSearchAndFilter?: boolean;
}) => {
  const moreButtonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  return (
    <Flex>
      <MenuLogin
        width={width}
        inputType={MenuInputType.FILTER}
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
        disableSearchAndFilter={disableSearchAndFilter}
        placeholder={placeholder}
      />
      <ButtonBorder
        setRef={moreButtonRef}
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

type MenuItemComponent = ReactElement<typeof MenuItem>;
