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
import * as Cards from 'design/CardError';
import { Indicator, Box } from 'design';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { useTeleport } from 'teleport/teleportContextProvider';
import NodeList from 'teleport/components/NodeList';
import useClusterNodes from './useClusterNodes';

export default function ClusterNodes() {
  const teleCtx = useTeleport();
  const state = useClusterNodes(teleCtx);
  return <Nodes {...state} />;
}

export function Nodes({
  nodes,
  getNodeLoginOptions,
  startSshSession,
  attempt,
}: NodesProp) {
  if (attempt.isFailed) {
    return <Cards.Failed alignSelf="baseline" message={attempt.message} />;
  }

  function onLoginSelect(e: React.MouseEvent, login: string, serverId: string) {
    e.preventDefault();
    startSshSession(login, serverId);
  }

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle mr="5">Nodes</FeatureHeaderTitle>
      </FeatureHeader>
      {attempt.isProcessing && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.isSuccess && (
        <NodeList
          onLoginMenuOpen={getNodeLoginOptions}
          nodes={nodes}
          onLoginSelect={onLoginSelect}
        />
      )}
    </FeatureBox>
  );
}

type NodesProp = ReturnType<typeof useClusterNodes>;
