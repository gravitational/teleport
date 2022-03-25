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
import Empty, { EmptyStateInfo } from 'teleport/components/Empty';
import NodeList from 'teleport/components/NodeList';
import useTeleport from 'teleport/useTeleport';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useNodes, { State } from './useNodes';
import AddNode from './AddNode';
import ButtonAdd from './ButtonAdd';

export default function Container() {
  const teleCtx = useTeleport();
  const stickyCluster = useStickyClusterId();
  const state = useNodes(teleCtx, stickyCluster);
  return <Nodes {...state} />;
}

export function Nodes(props: State) {
  const {
    nodes,
    getNodeLoginOptions,
    startSshSession,
    attempt,
    showAddNode,
    canCreate,
    hideAddNode,
    isLeafCluster,
    isAddNodeVisible,
    clusterId,
  } = props;

  function onLoginSelect(e: React.MouseEvent, login: string, serverId: string) {
    e.preventDefault();
    startSshSession(login, serverId);
  }

  function onSshEnter(login: string, serverId: string) {
    startSshSession(login, serverId);
  }

  const isEmpty = attempt.status === 'success' && nodes.length === 0;
  const hasNodes = attempt.status === 'success' && nodes.length > 0;

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <FeatureHeaderTitle>Servers</FeatureHeaderTitle>
        {hasNodes && (
          <Flex alignItems="center">
            {hasNodes && (
              <QuickLaunch width="280px" onPress={onSshEnter} mr={3} />
            )}

            <ButtonAdd
              isLeafCluster={isLeafCluster}
              canCreate={canCreate}
              onClick={showAddNode}
            />
          </Flex>
        )}
      </FeatureHeader>
      {attempt.status === 'failed' && <Danger>{attempt.statusText} </Danger>}
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {hasNodes && (
        <>
          <NodeList
            nodes={nodes}
            onLoginMenuOpen={getNodeLoginOptions}
            onLoginSelect={onLoginSelect}
          />
        </>
      )}
      {isEmpty && (
        <Empty
          clusterId={clusterId}
          canCreate={canCreate && !isLeafCluster}
          onClick={showAddNode}
          emptyStateInfo={emptyStateInfo}
        />
      )}
      {isAddNodeVisible && <AddNode onClose={hideAddNode} />}
    </FeatureBox>
  );
}

const emptyStateInfo: EmptyStateInfo = {
  title: 'Add your first Linux server to Teleport',
  byline:
    'Teleport Server Access consolidates SSH access across all environments.',
  docsURL: 'https://goteleport.com/docs/server-access/getting-started/',
  resourceType: 'server',
  readOnly: {
    title: 'No Servers Found',
    resource: 'servers',
  },
};
