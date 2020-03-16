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
import { withState } from 'shared/hooks';
import cfg from 'teleport/config';
import AjaxPoller from 'teleport/components/AjaxPoller';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { useStoreUser, useStoreNodes } from 'teleport/teleportContextProvider';
import NodeList from 'teleport/components/NodeList';
import { Node } from 'teleport/services/nodes';
import history from 'teleport/services/history';

const POLLING_INTERVAL = 10000; // every 10 sec

type NodesProps = {
  nodes: Node[];
  logins: string[];
  onFetch: () => Promise<Node[]>;
};

export function Nodes({ nodes, logins, onFetch }: NodesProps) {
  function onLoginMenuSelect(login: string, serverId: string) {
    const url = cfg.getSshConnectRoute({ login, serverId });
    history.push(url);
  }

  function onLoginMenuOpen(serverId: string) {
    return logins.map(login => {
      const url = cfg.getSshConnectRoute({
        serverId,
        login,
      });

      return {
        login,
        url,
      };
    });
  }

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle mr="5">Nodes</FeatureHeaderTitle>
      </FeatureHeader>
      <NodeList
        onLoginMenuOpen={onLoginMenuOpen}
        onLoginSelect={onLoginMenuSelect}
        nodes={nodes}
      />
      <AjaxPoller time={POLLING_INTERVAL} onFetch={onFetch} />
    </FeatureBox>
  );
}

const mapState = () => {
  const store = useStoreNodes();
  const storeUser = useStoreUser();
  return {
    onFetch: () => store.fetchNodes(),
    logins: storeUser.getLogins(),
    nodes: store.state,
  };
};

export default withState(mapState)(Nodes);
