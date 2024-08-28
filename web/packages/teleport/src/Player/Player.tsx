/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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

const validRecordingTypes = ['ssh', 'k8s', 'desktop', 'database'];

export function Player() {
  const { sid, clusterId } = useParams<UrlPlayerParams>();
  const { search } = useLocation();

  const recordingType = getUrlParameter(
    'recordingType',
    search
  ) as RecordingType;
  const durationMs = Number(getUrlParameter('durationMs', search));

  const validRecordingType = validRecordingTypes.includes(recordingType);
  const validDurationMs = Number.isInteger(durationMs) && durationMs > 0;

  document.title = `Play ${sid} • ${clusterId}`;

  function onLogout() {
    session.logout();
  }

  if (!validRecordingType) {
    return (
      <StyledPlayer>
        <Box textAlign="center" mx={10} mt={5}>
          <Danger mb={0}>
            Invalid query parameter recordingType: {recordingType}, should be
            one of {validRecordingTypes.join(', ')}.
          </Danger>
        </Box>
      </StyledPlayer>
    );
  }

  if (!validDurationMs) {
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
          <SshPlayer sid={sid} clusterId={clusterId} durationMs={durationMs} />
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
