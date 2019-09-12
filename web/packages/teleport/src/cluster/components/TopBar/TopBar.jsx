/*
Copyright 2019 Gravitational, Inc.

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
import { NavLink } from 'react-router-dom';
import { withState } from 'shared/hooks';
import { MenuItemIcon, MenuItem } from 'design/Menu/';
import TopNavUserMenu from 'design/TopNav/TopNavUserMenu';
import { Flex, ButtonPrimary, TopNav } from 'design';
import session from 'teleport/services/session';
import { useStoreUser, useStoreNav } from 'teleport/teleport';

export class TopBar extends React.Component {
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
    const { username, navItems, pl } = this.props;
    const { open } = this.state;
    const $items = navItems.map((item, index) => (
      <MenuItem {...this.menuItemProps} key={index} to={item.to}>
        <MenuItemIcon as={item.Icon} mr="2" />
        {item.title}
      </MenuItem>
    ));

    return (
      <TopNav pl={pl} height="72px" bg="transparent">
        <Flex ml="auto" height="100%">
          <TopNavUserMenu
            menuListCss={menuListCss}
            open={open}
            onShow={this.onShowMenu}
            onClose={this.onCloseMenu}
            user={username}
          >
            {$items}
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
  const navItems = useStoreNav().getTopMenuItems();
  const { username } = useStoreUser().state;
  return {
    navItems,
    username,
    onLogout: () => session.logout(),
  };
}

export default withState(mapState)(TopBar);

{
  /* <Box width="200px">
          <Select
            isSimpleValue={true}
            isSearchable={true}
            value={value}
            onChange={onChange}
            options={options}
            placeholder="Selector cluster"
          />
        </Box> */
}
