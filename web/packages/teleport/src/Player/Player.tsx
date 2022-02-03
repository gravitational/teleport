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
import { useParams, useLocation } from 'teleport/components/Router';
import { Flex, Box } from 'design';
import Tabs, { TabItem } from './PlayerTabs';
import SshPlayer from './SshPlayer';
import { DesktopPlayer } from './DesktopPlayer';
import ActionBar from './ActionBar';
import session from 'teleport/services/session';
import { colors } from 'teleport/Console/colors';
import { UrlPlayerParams } from 'teleport/config';
import { getUrlParameter } from 'teleport/services/history';
import { Danger } from 'design/Alert';
import { RecordingType } from 'teleport/services/recordings';

export default function Player() {
  const { sid, clusterId } = useParams<UrlPlayerParams>();

  const recordingType = getUrlParameter(
    'recordingType',
    useLocation().search
  ) as RecordingType;

  document.title = `${clusterId} • Play ${sid}`;

  function onLogout() {
    session.logout();
  }

  if (recordingType !== 'ssh' && recordingType !== 'desktop') {
    return (
      <StyledPlayer>
        <Box textAlign="center" m={10}>
          <Danger>
            {' '}
            `Invalid recording type: {recordingType}, should be 'ssh' or
            'desktop'`{' '}
          </Danger>
        </Box>
      </StyledPlayer>
    );
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
        bg={colors.bgTerminal}
        flex="1"
        style={{
          overflow: 'auto',
          position: 'relative',
        }}
      >
        {recordingType === 'ssh' && (
          <SshPlayer sid={sid} clusterId={clusterId} />
        )}

        {recordingType === 'desktop' && (
          <DesktopPlayer sid={sid} clusterId={clusterId} />
        )}
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

  .terminal .xterm-viewport {
    overflow-y: hidden !important;
  }
`;
