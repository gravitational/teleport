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

import React from 'react'
import { NavLink } from 'react-router-dom';
import { useFluxStore } from 'gravity/components/nuclear';
import { withState } from 'shared/hooks';
import userGetters from 'gravity/flux/user/getters';
import { getters as navGetters } from 'gravity/cluster/flux/nav';
import { getters as infoGetters } from 'gravity/cluster/flux/info';
import { fetchSiteInfo, changeRemoteAccess } from 'gravity/cluster/flux/info/actions';
import AjaxPoller from 'gravity/components/AjaxPoller';
import session from 'gravity/services/session';
import TopNavUserMenu from 'design/TopNav/TopNavUserMenu'
import { Flex, Text, ButtonOutlined, ButtonPrimary, TopNav } from 'design';
import { MenuItemIcon, MenuItem } from 'design/Menu/';
import ClusterStatus from './ClusterStatus';
import InfoDialog from './ClusterInfoDialog';
import RemoteAccess from './RemoteAccess';

const POLLING_INTERVAL = 5000; // every 5 sec

export class TopBar extends React.Component {

  state = {
    open: false,
    infoDialogOpen: false,
  };

  onShowInfoDialog = () => {
    this.setState({infoDialogOpen: true})
  }

  onCloseInfoDialog = () => {
    this.setState({infoDialogOpen: false})
  }

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

  menuItemProps = {
    onClick: this.onCloseMenu,
    py: 2,
    as: NavLink,
    exact: true
  }

  render() {
    const { user, info, remoteAccess, onRefresh, navItems, pl, onChangeRemoteAccess } = this.props;
    const { open, infoDialogOpen } = this.state;
    const username = user.userId;
    const { status, tshLogin, publicUrls, internalUrls } = info;
    const clusterPublicUrl = publicUrls[0];

    const $items = navItems.map( (item, index) => (
      <MenuItem {...this.menuItemProps} key={index} to={item.to}>
        <MenuItemIcon as={item.Icon} mr="2" />
          {item.title}
      </MenuItem>
    ))

    return (
      <TopNav pl={pl} height="72px" bg="transparent">
        <Flex alignItems="center">
          <ClusterStatus value={status} />
          <Text mx="3" typography="body2" color="text.primary">
            {clusterPublicUrl}
          </Text>
        </Flex>
        <ButtonOutlined width="120px" size="small" onClick={this.onShowInfoDialog}>
          View Info
        </ButtonOutlined>
        <Flex ml="auto" height="100%">
          <RemoteAccess remoteAccess={remoteAccess} onChange={onChangeRemoteAccess} />
          <TopNavUserMenu
            menuListCss={menuListCss}
            open={open}
            onShow={this.onShowMenu}
            onClose={this.onCloseMenu}
            user={username}>
            {$items}
            <MenuItem>
              <ButtonPrimary my={3} block onClick={this.onLogout}>
                Sign Out
              </ButtonPrimary>
            </MenuItem>
          </TopNavUserMenu>
          { infoDialogOpen && (
            <InfoDialog
              cmd={tshLogin}
              publicUrls={publicUrls}
              internalUrls={internalUrls}
              onClose={this.onCloseInfoDialog}/>
          )}
        </Flex>
        <AjaxPoller time={POLLING_INTERVAL} onFetch={onRefresh} />
      </TopNav>
    )
  }
}

const menuListCss = () => `
  width: 250px;
`

function mapState() {
  const user = useFluxStore(userGetters.user);
  const navStore = useFluxStore(navGetters.navStore);
  const infoStore = useFluxStore(infoGetters.infoStore);
  return {
    user,
    navItems: navStore.topNav,
    info: infoStore.info,
    remoteAccess: infoStore.remoteAccess,
    onLogout: () => session.logout(),
    onRefresh: fetchSiteInfo,
    onChangeRemoteAccess: changeRemoteAccess
  }
}

export default withState(mapState)(TopBar);