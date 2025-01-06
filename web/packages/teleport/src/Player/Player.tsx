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

import { useCallback, useEffect } from 'react';
import styled from 'styled-components';

import { Box, Flex, Indicator } from 'design';
import { Danger } from 'design/Alert';
import { makeSuccessAttempt, useAsync } from 'shared/hooks/useAsync';

import { useLocation, useParams } from 'teleport/components/Router';
import { UrlPlayerParams } from 'teleport/config';
import { getUrlParameter } from 'teleport/services/history';
import { RecordingType } from 'teleport/services/recordings';
import session from 'teleport/services/websession';
import useTeleport from 'teleport/useTeleport';

import ActionBar from './ActionBar';
import { DesktopPlayer } from './DesktopPlayer';
import Tabs, { TabItem } from './PlayerTabs';
import SshPlayer from './SshPlayer';

const validRecordingTypes = ['ssh', 'k8s', 'desktop', 'database'];

export function Player() {
  const ctx = useTeleport();
  const { sid, clusterId } = useParams<UrlPlayerParams>();
  const { search } = useLocation();

  useEffect(() => {
    document.title = `Play ${sid} • ${clusterId}`;
  }, [sid, clusterId]);

  const recordingType = getUrlParameter(
    'recordingType',
    search
  ) as RecordingType;

  // In order to render the progress bar, we need to know the length of the session.
  // All in-product links to the session player should include the session duration in the URL.
  // Some users manually build the URL based on the session ID and don't specify the session duration.
  // For those cases, we make a separate API call to get the duration.
  const [fetchDurationAttempt, fetchDuration] = useAsync(
    useCallback(
      () => ctx.recordingsService.fetchRecordingDuration(clusterId, sid),
      [ctx.recordingsService, clusterId, sid]
    )
  );

  const validRecordingType = validRecordingTypes.includes(recordingType);
  const durationMs = Number(getUrlParameter('durationMs', search));
  const shouldFetchSessionDuration =
    validRecordingType && (!Number.isInteger(durationMs) || durationMs <= 0);

  useEffect(() => {
    if (shouldFetchSessionDuration) {
      fetchDuration();
    }
  }, [fetchDuration, shouldFetchSessionDuration]);

  const combinedAttempt = shouldFetchSessionDuration
    ? fetchDurationAttempt
    : makeSuccessAttempt({ durationMs });

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

  if (
    combinedAttempt.status === '' ||
    combinedAttempt.status === 'processing'
  ) {
    return (
      <StyledPlayer>
        <Box textAlign="center" mx={10} mt={5}>
          <Indicator />
        </Box>
      </StyledPlayer>
    );
  }
  if (combinedAttempt.status === 'error') {
    return (
      <StyledPlayer>
        <Box textAlign="center" mx={10} mt={5}>
          <Danger mb={0}>
            Unable to determine the length of this session. The session
            recording may be incomplete or corrupted.
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
            durationMs={combinedAttempt.data.durationMs}
          />
        ) : (
          <SshPlayer
            sid={sid}
            clusterId={clusterId}
            durationMs={combinedAttempt.data.durationMs}
          />
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
