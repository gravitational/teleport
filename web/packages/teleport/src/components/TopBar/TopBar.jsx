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
    const { username, topMenuItems, pl } = this.props;
    const { open } = this.state;
    const $userMenuItems = topMenuItems.map((item, index) => (
      <MenuItem {...this.menuItemProps} key={index} to={item.to}>
        <MenuItemIcon as={item.Icon} mr="2" />
        {item.title}
      </MenuItem>
    ));

    return (
      <TopNav
        height="72px"
        pl={pl}
        style={{ zIndex: '1', boxShadow: '0 4px 16px rgba(0,0,0,.24)' }}
      >
        <TopNavItem width="192px" pr="5" as={Link} to={cfg.routes.app}>
          <Image
            src={teleportLogoSvg}
            mx="3"
            maxHeight="40px"
            maxWidth="160px"
          />
        </TopNavItem>
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
