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
import { NavLink, Link } from 'react-router-dom';
import styled from 'styled-components';
import { withState } from 'shared/hooks';
import { Image, SideNav, SideNavItem } from 'design';
import SideNavItemIcon from 'design/SideNav/SideNavItemIcon';
import teleportLogoSvg from 'design/assets/images/teleport-logo.svg';
import { useStoreUser, useStoreNav } from 'teleport/teleport';
import cfg from 'teleport/config';

export function ClusterSideNav({ items, version, homeUrl }) {
  const $items = items.map((item, index) => (
    <SideNavItem key={index} as={NavLink} exact={item.exact} to={item.to}>
      <SideNavItemIcon as={item.Icon} />
      {item.title}
    </SideNavItem>
  ));

  return (
    <SideNav>
      <StyledLogoItem py="2" pl="5" as={Link} to={homeUrl}>
        <Image src={teleportLogoSvg} maxHeight="40px" maxWidth="120px" mr="3" />
        <span title={version}>{version}</span>
      </StyledLogoItem>
      <div
        style={{ display: 'flex', flexDirection: 'column', overflow: 'auto' }}
      >
        {$items}
      </div>
    </SideNav>
  );
}

const StyledLogoItem = styled(SideNavItem)`
  &:active {
    border-left-color: transparent;
    color: ${({ theme }) => theme.colors.text.primary};
  }

  > span {
    line-height: 1.4;
    text-overflow: ellipsis;
    overflow: hidden;
  }
`;

export default withState(() => {
  const items = useStoreNav().getSideItems();
  const { version } = useStoreUser().state;
  return {
    version,
    items,
    homeUrl: cfg.getClusterRoute(),
  };
})(ClusterSideNav);
