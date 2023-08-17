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

import { Flex, Box } from 'design';

import { Danger } from 'design/Alert';

import { useParams, useLocation } from 'teleport/components/Router';

import session from 'teleport/services/websession';
import { UrlPlayerParams } from 'teleport/config';
import { getUrlParameter } from 'teleport/services/history';

import { RecordingType } from 'teleport/services/recordings';

import ActionBar from './ActionBar';
import { DesktopPlayer } from './DesktopPlayer';
import SshPlayer from './SshPlayer';
import Tabs, { TabItem } from './PlayerTabs';

export default function Player() {
  const { sid, clusterId } = useParams<UrlPlayerParams>();
  const { search } = useLocation();

  const recordingType = getUrlParameter(
    'recordingType',
    search
  ) as RecordingType;
  const durationMs = Number(getUrlParameter('durationMs', search));

  const validRecordingType =
    recordingType === 'ssh' ||
    recordingType === 'k8s' ||
    recordingType === 'desktop';
  const validDurationMs = Number.isInteger(durationMs) && durationMs > 0;

  document.title = `${clusterId} • Play ${sid}`;

  function onLogout() {
    session.logout();
  }

  if (!validRecordingType) {
    return (
      <StyledPlayer>
        <Box textAlign="center" mx={10} mt={5}>
          <Danger mb={0}>
            Invalid query parameter recordingType: {recordingType}, should be
            'ssh' or 'desktop'
          </Danger>
        </Box>
      </StyledPlayer>
    );
  }

  if (recordingType === 'desktop' && !validDurationMs) {
    return (
      <StyledPlayer>
        <Box textAlign="center" mx={10} mt={5}>
          <Danger mb={0}>
            Invalid query parameter durationMs:{' '}
            {getUrlParameter('durationMs', search)}, should be an integer.
          </Danger>
        </Box>
      </StyledPlayer>
    );
  }

  return (
    <StyledPlayer>
      <Flex bg="levels.surface" height="38px">
        <Tabs flex="1 0">
          <TabItem title="Session Player" />
        </Tabs>
        <ActionBar onLogout={onLogout} />
      </Flex>
      <Flex
        flex="1"
        style={{
          overflow: 'auto',
          position: 'relative',
        }}
      >
        {recordingType === 'desktop' ? (
          <DesktopPlayer
            sid={sid}
            clusterId={clusterId}
            durationMs={durationMs}
          />
        ) : (
          <SshPlayer sid={sid} clusterId={clusterId} />
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
