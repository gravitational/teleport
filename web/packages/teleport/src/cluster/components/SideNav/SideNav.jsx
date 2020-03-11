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
import { SideNav, SideNavItem } from 'design';
import SideNavItemIcon from 'design/SideNav/SideNavItemIcon';
import { useStoreUser, useStoreNav } from 'teleport/teleportContextProvider';
import cfg from 'teleport/config';

export function ClusterSideNav({ items }) {
  const $items = items.map((item, index) => (
    <SideNavItem key={index} as={NavLink} exact={item.exact} to={item.to}>
      <SideNavItemIcon as={item.Icon} />
      {item.title}
    </SideNavItem>
  ));

  return <SideNav>{$items}</SideNav>;
}

export default withState(() => {
  const items = useStoreNav().getSideItems();
  const { version } = useStoreUser().state;

  return {
    version,
    items,
    clusterName: cfg.clusterName,
    homeUrl: cfg.getClusterRoute(),
  };
})(ClusterSideNav);
