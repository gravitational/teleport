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
import { Indicator, Box, Flex } from 'design';
import { Danger } from 'design/Alert';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import QuickLaunch from 'teleport/components/QuickLaunch';
import InputSearch from 'teleport/components/InputSearch';
import NodeList from 'teleport/components/NodeList';
import useTeleport from 'teleport/useTeleport';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useNodes, { State } from './useNodes';
import AddNode from './AddNode';
import ButtonAdd from './ButtonAdd';

export default function Container() {
  const teleCtx = useTeleport();
  const { clusterId } = useStickyClusterId();
  const state = useNodes(teleCtx, clusterId);
  return <Nodes {...state} />;
}

export function Nodes(props: State) {
  const {
    nodes,
    searchValue,
    setSearchValue,
    getNodeLoginOptions,
    startSshSession,
    attempt,
    showAddNode,
    canCreate,
    hideAddNode,
    isAddNodeVisible,
    isEnterprise,
  } = props;

  function onLoginSelect(e: React.MouseEvent, login: string, serverId: string) {
    e.preventDefault();
    startSshSession(login, serverId);
  }

  function onSshEnter(login: string, serverId: string) {
    startSshSession(login, serverId);
  }

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <FeatureHeaderTitle>Servers</FeatureHeaderTitle>
        <ButtonAdd
          isEnterprise={isEnterprise}
          canCreate={canCreate}
          onClick={showAddNode}
        />
      </FeatureHeader>
      <Flex
        mb={4}
        alignItems="center"
        flex="0 0 auto"
        justifyContent="space-between"
      >
        <InputSearch mr="3" onChange={setSearchValue} />
        <QuickLaunch width="280px" onPress={onSshEnter} />
      </Flex>
      {attempt.status === 'failed' && <Danger>{attempt.statusText} </Danger>}
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'success' && (
        <NodeList
          nodes={nodes}
          searchValue={searchValue}
          onLoginMenuOpen={getNodeLoginOptions}
          onLoginSelect={onLoginSelect}
        />
      )}
      {isAddNodeVisible && <AddNode onClose={hideAddNode} />}
    </FeatureBox>
  );
}
