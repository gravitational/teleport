/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { NavLink } from 'react-router-dom';

import { ButtonPrimary, Box } from 'design';
import { OpenBox, Person } from 'design/Icon';
import TopNavUserMenu from 'design/TopNav/TopNavUserMenu';
import { MenuItemIcon, MenuItem } from 'design/Menu';

import cfg from 'teleport/config';
import { NavItem } from 'teleport/stores/storeNav';

export function UserMenuNav({ navItems, username, logout }: Props) {
  const [open, setOpen] = React.useState(false);
  const menuItemProps = {
    onClick: closeMenu,
    py: 2,
    as: NavLink,
    exact: true,
  };

  const $userMenuItems = navItems.map((item, index) => (
    <MenuItem {...menuItemProps} key={index} to={item.getLink()}>
      <MenuItemIcon as={item.Icon} mr="2" />
      {item.title}
    </MenuItem>
  ));

  function showMenu() {
    setOpen(true);
  }

  function closeMenu() {
    setOpen(false);
  }

  function handleLogout() {
    closeMenu();
    logout();
  }

  return (
    <TopNavUserMenu
      menuListCss={menuListCss}
      open={open}
      onShow={showMenu}
      onClose={closeMenu}
      user={username}
    >
      <MenuItem {...menuItemProps} to={cfg.routes.root}>
        <MenuItemIcon as={Person} mr="2" />
        Access Provider
      </MenuItem>
      <MenuItem {...menuItemProps} to={cfg.routes.discover}>
        <MenuItemIcon as={OpenBox} mr="2" />
        Access Manager
      </MenuItem>
      <Box
        my={2}
        css={`
          border-bottom: 1px solid #e3e3e3;
        `}
      />
      {$userMenuItems}
      <MenuItem>
        <ButtonPrimary my={3} block onClick={handleLogout}>
          Sign Out
        </ButtonPrimary>
      </MenuItem>
    </TopNavUserMenu>
  );
}

const menuListCss = () => `
  width: 250px;
`;

type Props = {
  navItems: NavItem[];
  username: string;
  logout(): void;
};
