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

import { useState, useEffect, useCallback } from 'react';
import { useAttempt } from 'shared/hooks';
import TeleportContext from 'teleport/teleportContext';
import cfg from 'teleport/config';
import { Node } from 'teleport/services/nodes';

export default function useNodes(ctx: TeleportContext, clusterId: string) {
  const canCreate = ctx.storeUser.getTokenAccess().create;
  const [nodes, setNodes] = useState<Node[]>([]);
  const [searchValue, setSearchValue] = useState('');
  const [attempt, attemptActions] = useAttempt({ isProcessing: true });
  const [isAddNodeVisible, setIsAddNodeVisible] = useState(false);
  const logins = ctx.storeUser.getLogins();
  const isEnterprise = ctx.isEnterprise;

  useEffect(() => {
    fetchNodes();
  }, [clusterId]);

  const getNodeLoginOptions = useCallback(
    (serverId: string) => makeOptions(clusterId, serverId, logins),
    [logins]
  );

  const openNewTab = (url: string) => {
    const element = document.createElement('a');
    element.setAttribute('href', `${url}`);
    // works in ie11
    element.setAttribute('target', `_blank`);
    element.style.display = 'none';
    document.body.appendChild(element);
    element.click();
    document.body.removeChild(element);
  };

  const startSshSession = (login: string, serverId: string) => {
    const url = cfg.getSshConnectRoute({
      clusterId,
      serverId,
      login,
    });

    openNewTab(url);
  };

  const fetchNodes = () => {
    attemptActions.do(() =>
      ctx.nodeService.fetchNodes(clusterId).then(setNodes)
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
    searchValue,
    setSearchValue,
    attempt,
    nodes,
    getNodeLoginOptions,
    startSshSession,
    isAddNodeVisible,
    isEnterprise,
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
