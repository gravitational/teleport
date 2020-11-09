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
import { NavLink } from 'react-router-dom';
import TopNavUserMenu from 'design/TopNav/TopNavUserMenu';
import { MenuItemIcon, MenuItem } from 'design/Menu';
import { Text, Flex, ButtonPrimary, TopNav } from 'design';
import ClusterSelector from './ClusterSelector';
import { useTheme } from 'styled-components';
import useTeleport from 'teleport/useTeleport';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useTopBar from './useTopBar';

export default function Container() {
  const ctx = useTeleport();
  const stickCluster = useStickyClusterId();
  const state = useTopBar(ctx, stickCluster);
  return <TopBar {...state} />;
}

export function TopBar(props: ReturnType<typeof useTopBar>) {
  const {
    username,
    loadClusters,
    popupItems,
    changeCluster,
    clusterId,
    hasClusterUrl,
  } = props;

  const theme = useTheme();
  const [open, setOpen] = React.useState(false);
  const menuItemProps = {
    onClick: closeMenu,
    py: 2,
    as: NavLink,
    exact: true,
  };

  const $userMenuItems = popupItems.map((item, index) => (
    <MenuItem {...menuItemProps} key={index} to={item.getLink(clusterId)}>
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

  function logout() {
    closeMenu();
    props.logout();
  }

  // instead of re-creating an expensive react-select component,
  // hide/show it instead
  const styles = {
    display: !hasClusterUrl ? 'none' : 'block',
  };

  return (
    <TopNav
      height="56px"
      bg="inherit"
      pl="6"
      style={{
        overflowY: 'initial',
        flexShrink: '0',
        borderBottom: `1px solid ${theme.colors.primary.main}`,
      }}
    >
      {!hasClusterUrl && <Text typography="h2">{props.title}</Text>}
      <ClusterSelector
        value={clusterId}
        width="384px"
        maxMenuHeight={200}
        mr="20px"
        onChange={changeCluster}
        onLoad={loadClusters}
        style={styles}
      />
      <Flex ml="auto" height="100%">
        <TopNavUserMenu
          menuListCss={menuListCss}
          open={open}
          onShow={showMenu}
          onClose={closeMenu}
          user={username}
        >
          {$userMenuItems}
          <MenuItem>
            <ButtonPrimary my={3} block onClick={logout}>
              Sign Out
            </ButtonPrimary>
          </MenuItem>
        </TopNavUserMenu>
      </Flex>
    </TopNav>
  );
}

const menuListCss = () => `
  width: 250px;
`;
