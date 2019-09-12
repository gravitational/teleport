import React from 'react'
import { NavLink, Link } from 'react-router-dom';
import TopNavUserMenu from 'design/TopNav/TopNavUserMenu'
import hubLogo from 'design/assets/images/gravity-hub.svg';
import { Image, Flex, ButtonPrimary, TopNav, TopNavItem } from 'design';
import { MenuItem } from 'design/Menu/';
import cfg from 'e-gravity/config';

export default class HubTopNav extends React.Component {

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
  }

  onLogout = () => {
    this.onCloseMenu();
    this.props.onLogout();
  }

  render() {
    const { userName, items, pl } = this.props;
    const { open } = this.state;

    const $items = items.map((item, index) => (
      <TopNavItem px="5" as={NavLink} exact={item.exact} key={index} to={item.to}>
        {item.title}
      </TopNavItem>
    ))

    return (
      <TopNav height="72px" pl={pl} style={{ "zIndex": "1", "boxShadow": "0 4px 16px rgba(0,0,0,.24)" }}>
        <TopNavItem pr="5" as={Link} to={cfg.routes.defaultEntry}>
          <Image src={hubLogo}  ml="3" mr="4" maxHeight="40px" maxWidth="160px" />
        </TopNavItem>
        {$items}
        <Flex ml="auto" height="100%">
          <TopNavUserMenu
            menuListCss={menuListCss}
            open={open}
            onShow={this.onShowMenu}
            onClose={this.onCloseMenu}
            user={userName}>
            <MenuItem>
              <ButtonPrimary my={3} block onClick={this.onLogout}>
                Sign Out
              </ButtonPrimary>
            </MenuItem>
          </TopNavUserMenu>
        </Flex>
      </TopNav>
    )
  }
}

const menuListCss = () => `
  width: 250px;
`