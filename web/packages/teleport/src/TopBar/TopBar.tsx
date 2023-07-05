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
import styled from 'styled-components';
import { Text, Flex, TopNav } from 'design';

import useTeleport from 'teleport/useTeleport';
import useStickyClusterId from 'teleport/useStickyClusterId';
import { UserMenuNav } from 'teleport/components/UserMenuNav';

import ClusterSelector from './ClusterSelector';
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

  // instead of re-creating an expensive react-select component,
  // hide/show it instead
  const styles = {
    display: !hasClusterUrl ? 'none' : 'block',
  };

  return (
    <TopBarContainer>
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
        <UserMenuNav
          navItems={popupItems}
          username={username}
          logout={props.logout}
        />
      </Flex>
    </TopBarContainer>
  );
}

export const TopBarContainer = styled(TopNav)`
  height: 56px;
  background-color: inherit;
  padding-left: ${({ theme }) => `${theme.space[6]}px`};
  overflow-y: initial;
  flex-shrink: 0;
  border-bottom: 1px solid
    ${({ theme }) => theme.colors.levels.surfaceSecondary};
`;
