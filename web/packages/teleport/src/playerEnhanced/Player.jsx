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
import {
  Redirect,
  Route,
  useParams,
  useRouteMatch,
} from 'teleport/components/Router';
import { Flex } from 'design';
import cfg from 'teleport/config';
import Tabs, { TabItem } from './PlayerTabs';
import { colors } from 'teleport/console/components/colors';
import SshPlayer from './SshPlayer';
import BpfLogs from './BpfLogs';

export default function SessionAudit() {
  const { sid, clusterId } = useParams();

  React.useState(() => {
    cfg.setClusterId(clusterId);
  }, []);

  const indexRoute = cfg.getPlayerRoute({ sid });
  const playerRoute = cfg.getSessionAuditPlayerRoute({ sid });
  const cmdsRoute = cfg.getSessionAuditCmdsRoute({ sid });

  const isCmdTabActive = Boolean(useRouteMatch(cfg.routes.sessionAuditCmds));
  const isPlayerTabActive = Boolean(
    useRouteMatch(cfg.routes.sessionAuditPlayer)
  );

  return (
    <StyledPlayer>
      <Tabs>
        <TabItem to={playerRoute} title="Player" />
        <TabItem to={cmdsRoute} title="Commands" />
      </Tabs>
      <StyledDocument visible={isPlayerTabActive}>
        <SshPlayer sid={sid} clusterId={clusterId} />
      </StyledDocument>
      <StyledDocument visible={isCmdTabActive}>
        <BpfLogs />
      </StyledDocument>
      <Route exact path={indexRoute}>
        <Redirect to={playerRoute} />
      </Route>
    </StyledPlayer>
  );
}

const StyledPlayer = styled.div`
  display: flex;
  height: 100%;
  width: 100%;
  position: absolute;
  flex-direction: column;
`;

function StyledDocument({ children, visible }) {
  return (
    <Flex
      bg={colors.bgTerminal}
      flex="1"
      style={{
        overflow: 'auto',
        display: visible ? 'flex' : 'none',
        position: 'relative',
      }}
    >
      {children}
    </Flex>
  );
}
