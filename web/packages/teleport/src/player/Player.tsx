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
import { useParams } from 'shared/components/Router';
import { Flex } from 'design';
import cfg from 'teleport/config';
import Tabs, { TabItem } from './PlayerTabs';
import SshPlayer from './SshPlayer';
import ActionBar from './ActionBar';
import session from 'teleport/services/session';
import { colors } from 'teleport/console/components/colors';

export default function Player() {
  const { sid, clusterId } = useParams();

  React.useMemo(() => {
    cfg.setClusterId(clusterId);
  }, [clusterId]);

  function onLogout() {
    session.logout();
  }

  return (
    <StyledPlayer>
      <Flex bg={colors.primary.light} height="38px">
        <Tabs flex="1 0">
          <TabItem title="Session Player" />
        </Tabs>
        <ActionBar onLogout={onLogout} />
      </Flex>
      <Flex
        bg={colors.primary.dark}
        flex="1"
        style={{
          overflow: 'auto',
          position: 'relative',
        }}
      >
        <SshPlayer sid={sid} clusterId={clusterId} />
      </Flex>
    </StyledPlayer>
  );
}
const StyledPlayer = styled.div`
  display: flex;
  height: 100%;
  width: 100%;
  position: absolute;
  flex-direction: column;

  // always render cursor as focused
  .terminal:not(.focus) .terminal-cursor {
    outline: none !important;
    background-color: #fff;
  }

  .terminal .xterm-viewport {
    overflow-y: hidden !important;
  }
`;
