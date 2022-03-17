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
import styled from 'styled-components';
import { Flex } from 'design';
import ClusterNavButton from 'teleterm/ui/DocumentCluster/ClusterNavButton';
import ClusterContext, {
  NavLocation,
  useClusterContext,
} from 'teleterm/ui/DocumentCluster/clusterContext';

export default function SideNav(props: Props) {
  const ctx = useClusterContext();
  const items = createItems(ctx);

  const $items = items.map((item, index) => {
    return (
      <ClusterNavButton
        p={1}
        my={1}
        title={item.title}
        to={item.to}
        key={index}
      />
    );
  });

  return <StyledNav {...props}>{$items}</StyledNav>;
}

type Props = {
  [index: string]: any;
};

export type SideNavItem = {
  to: NavLocation;
  title: string;
};

const StyledNav = styled(Flex)`
  min-width: 180px;
  overflow: auto;
  height: 100%;
`;

function createItems(ctx: ClusterContext): SideNavItem[] {
  const serverCount = ctx.getServers().length;
  const dbCount = ctx.getDbs().length;
  const kubeCount = ctx.getKubes().length;
  const appCount = ctx.getApps().length;

  return [
    {
      to: '/resources/servers',
      title: `Servers (${serverCount})`,
    },
    {
      to: '/resources/databases',
      title: `Databases (${dbCount})`,
    },
    {
      to: '/resources/kubes',
      title: `Kubes (${kubeCount})`,
    },
    {
      to: '/resources/apps',
      title: `Apps (${appCount})`,
    },
  ];
}
