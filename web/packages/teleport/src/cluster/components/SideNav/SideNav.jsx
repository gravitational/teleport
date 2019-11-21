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
import { useStoreClusters, useStoreUser, useStoreNav } from 'teleport/teleport';
import history from 'teleport/services/history';
import cfg from 'teleport/config';
import SelectCluster from './SelectCluster';

export function ClusterSideNav({
  items,
  clusterName,
  clusterOptions,
  onChangeCluster,
}) {
  const $items = items.map((item, index) => (
    <SideNavItem key={index} as={NavLink} exact={item.exact} to={item.to}>
      <SideNavItemIcon as={item.Icon} />
      {item.title}
    </SideNavItem>
  ));

  const selectedCluster = {
    value: clusterName,
    label: clusterName,
  };
  return (
    <SideNav>
      <SelectCluster
        py="2"
        px="3"
        value={selectedCluster}
        options={clusterOptions}
        onChange={onChangeCluster}
      />
      <div
        style={{ display: 'flex', flexDirection: 'column', overflow: 'auto' }}
      >
        {$items}
      </div>
    </SideNav>
  );
}

export default withState(() => {
  const clusterOptions = useStoreClusters().getClusterOptions();
  const items = useStoreNav().getSideItems();
  const { version } = useStoreUser().state;

  function onChangeCluster({ url }) {
    history.push(url);
  }

  return {
    clusterOptions,
    version,
    items,
    clusterName: cfg.clusterName,
    homeUrl: cfg.getClusterRoute(),
    onChangeCluster,
  };
})(ClusterSideNav);
