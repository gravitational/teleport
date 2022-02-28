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

import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import Ctx from 'teleport/teleportContext';
import { StickyCluster } from 'teleport/types';
import cfg from 'teleport/config';
import { Node } from 'teleport/services/nodes';
import { openNewTab } from 'teleport/lib/util';

export default function useNodes(ctx: Ctx, stickyCluster: StickyCluster) {
  const { isLeafCluster, clusterId } = stickyCluster;
  const [nodes, setNodes] = useState<Node[]>([]);
  const { attempt, run, setAttempt } = useAttempt('processing');
  const [isAddNodeVisible, setIsAddNodeVisible] = useState(false);
  const canCreate = ctx.storeUser.getTokenAccess().create;
  const logins = ctx.storeUser.getSshLogins();

  useEffect(() => {
    run(() =>
      ctx.nodeService.fetchNodes(clusterId).then(res => setNodes(res.nodes))
    );
  }, [clusterId]);

  const getNodeLoginOptions = (serverId: string) =>
    makeOptions(clusterId, serverId, logins);

  const startSshSession = (login: string, serverId: string) => {
    const url = cfg.getSshConnectRoute({
      clusterId,
      serverId,
      login,
    });

    openNewTab(url);
  };

  const fetchNodes = () => {
    return ctx.nodeService
      .fetchNodes(clusterId)
      .then(res => setNodes(res.nodes))
      .catch((err: Error) =>
        setAttempt({ status: 'failed', statusText: err.message })
      );
  };

  const hideAddNode = () => {
    setIsAddNodeVisible(false);
    fetchNodes();
  };

  const showAddNode = () => {
    setIsAddNodeVisible(true);
  };

  return {
    canCreate,
    attempt,
    nodes,
    getNodeLoginOptions,
    startSshSession,
    isAddNodeVisible,
    isLeafCluster,
    clusterId,
    hideAddNode,
    showAddNode,
  };
}

function makeOptions(
  clusterId: string,
  serverId = '',
  logins = [] as string[]
) {
  return logins.map(login => {
    const url = cfg.getSshConnectRoute({
      clusterId,
      serverId,
      login,
    });

    return {
      login,
      url,
    };
  });
}

export type State = ReturnType<typeof useNodes>;
