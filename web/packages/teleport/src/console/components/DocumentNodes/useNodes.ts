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

import { useEffect, useState } from 'react';
import { useAttempt } from 'shared/hooks';
import * as stores from './../../stores';
import { useConsoleContext } from 'teleport/console/consoleContextProvider';
import { Node } from 'teleport/services/nodes';

export default function useNodes({ clusterId, id }: stores.DocumentNodes) {
  const consoleCtx = useConsoleContext();
  const [attempt, attemptActions] = useAttempt({ isSuccess: true });
  const [nodes, setNodes] = useState<Node[]>([]);
  const [logins, setLogins] = useState<string[]>([]);

  useEffect(() => {
    attemptActions.do(() => {
      return consoleCtx.fetchNodes(clusterId).then(({ nodes, logins }) => {
        setLogins(logins);
        setNodes(nodes);
      });
    });
  }, [clusterId]);

  function createSshSession(login: string, serverId: string) {
    const url = consoleCtx.getSshDocumentUrl({
      serverId,
      login,
      clusterId,
    });
    consoleCtx.navigateTo({ url });
  }

  function changeCluster(value: string) {
    const clusterId = value;
    const url = consoleCtx.getNodeDocumentUrl(clusterId);
    consoleCtx.storeDocs.update(id, {
      url,
      clusterId,
    });

    consoleCtx.navigateTo({ url });
  }

  function getNodeSshLogins(serverId: string) {
    return logins.map(login => ({
      login,
      url: consoleCtx.getSshDocumentUrl({ serverId, login, clusterId }),
    }));
  }

  return {
    nodes,
    attempt,
    createSshSession,
    changeCluster,
    getNodeSshLogins,
  };
}
