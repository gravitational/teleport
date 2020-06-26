/*
Copyright 2019-2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { NavLink, Link } from 'react-router-dom';
import TopNavUserMenu from 'design/TopNav/TopNavUserMenu';
import { MenuItemIcon, MenuItem } from 'design/Menu';
import teleportLogoSvg from 'design/assets/images/teleport-logo.svg';
import { withState } from 'shared/hooks';
import session from 'teleport/services/session';
import { useStoreUser, useStoreNav } from 'teleport/teleportContextProvider';
import { Image, Flex, ButtonPrimary, TopNav, TopNavItem } from 'design';
import cfg from 'teleport/config';

export class DashboardTopNav extends React.Component {
  state = {
    open: false,
  };

  onShowMenu = () => {
    this.setState({ open: true });
  };

  onCloseMenu = () => {
    this.setState({ open: false });
  };

  onItemClick = () => {
    this.onClose();
  };

  onLogout = () => {
    this.onCloseMenu();
    this.props.onLogout();
  };

  menuItemProps = {
    onClick: this.onCloseMenu,
    py: 2,
    as: NavLink,
    exact: true,
  };

  render() {
    const { username, topMenuItems, pl, children } = this.props;
    const { open } = this.state;
    const $userMenuItems = topMenuItems.map((item, index) => (
      <MenuItem {...this.menuItemProps} key={index} to={item.to}>
        <MenuItemIcon as={item.Icon} mr="2" />
        {item.title}
      </MenuItem>
    ));

    return (
      <TopNav
        height="56px"
        pl={pl}
        style={{
          zIndex: '1',
          boxShadow: '0 4px 16px rgba(0,0,0,.24)',
          overflowX: 'auto',
        }}
      >
        <TopNavItem width="208px" as={Link} to={cfg.routes.app}>
          <Image
            src={teleportLogoSvg}
            mx="3"
            maxHeight="24px"
            maxWidth="160px"
          />
        </TopNavItem>
        {children}
        <Flex ml="auto" height="100%">
          <TopNavUserMenu
            menuListCss={menuListCss}
            open={open}
            onShow={this.onShowMenu}
            onClose={this.onCloseMenu}
            user={username}
          >
            {$userMenuItems}
            <MenuItem>
              <ButtonPrimary my={3} block onClick={this.onLogout}>
                Sign Out
              </ButtonPrimary>
            </MenuItem>
          </TopNavUserMenu>
        </Flex>
      </TopNav>
    );
  }
}

const menuListCss = () => `
  width: 250px;
`;

function mapState() {
  const topMenuItems = useStoreNav().getTopMenuItems();
  const { username } = useStoreUser().state;
  return {
    topMenuItems,
    username,
    onLogout: () => session.logout(),
  };
}

export default withState(mapState)(DashboardTopNav);
