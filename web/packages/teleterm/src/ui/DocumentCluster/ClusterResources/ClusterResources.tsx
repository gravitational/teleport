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

import { useClusterContext } from 'teleterm/ui/DocumentCluster/clusterContext';
import { ConnectMyComputerNavigationMenu } from 'teleterm/ui/ConnectMyComputer';

import SideNav from './SideNav';
import Servers from './Servers';
import Databases from './Databases';
import Kubes from './Kubes';

export default function ClusterResources() {
  const clusterCtx = useClusterContext();

  // subscribe to services
  const { navLocation } = clusterCtx.useState();
  clusterCtx.appCtx.clustersService.useState();

  React.useEffect(() => {
    if (clusterCtx.isLocationActive(`/resources/`, true)) {
      clusterCtx.changeLocation('/resources/servers');
    }
  }, [navLocation]);

  return (
    <StyledMain>
      <Flex pb={5} flexDirection="column">
        <Flex justifyContent="space-between">
          <SideNav mb={2} />
          <ConnectMyComputerNavigationMenu clusterUri={clusterCtx.clusterUri} />
        </Flex>
        <HorizontalSplit>
          {clusterCtx.isLocationActive('/resources/servers') && <Servers />}
          {clusterCtx.isLocationActive('/resources/databases') && <Databases />}
          {clusterCtx.isLocationActive('/resources/kubes') && <Kubes />}
        </HorizontalSplit>
      </Flex>
    </StyledMain>
  );
}

const StyledMain = styled.div`
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  flex: 1;
`;

const HorizontalSplit = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  min-width: 0;
`;
