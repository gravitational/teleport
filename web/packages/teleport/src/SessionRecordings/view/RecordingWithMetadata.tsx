/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { format } from 'date-fns';
import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';

import Flex from 'design/Flex';
import { ChevronLeft, Terminal } from 'design/Icon';
import { H3 } from 'design/Text';

import cfg from 'teleport/config';
import { useSuspenseGetRecordingMetadata } from 'teleport/services/recordings/hooks';
import { formatSessionRecordingDuration } from 'teleport/SessionRecordings/list/RecordingItem';
import { RecordingPlayer } from 'teleport/SessionRecordings/view/RecordingPlayer';

export type SummarySlot = (sessionId: string) => ReactNode;

interface RecordingWithMetadataProps {
  clusterId: string;
  sessionId: string;
  summarySlot?: SummarySlot;
}

export function RecordingWithMetadata({
  clusterId,
  sessionId,
  summarySlot,
}: RecordingWithMetadataProps) {
  const { data } = useSuspenseGetRecordingMetadata({
    clusterId,
    sessionId,
  });

  const [sidebarVisible, setSidebarVisible] = useState(true);

  const toggleSidebar = useCallback(() => {
    setSidebarVisible(prev => !prev);
  }, []);

  const summary = useMemo(
    () => summarySlot?.(sessionId),
    [summarySlot, sessionId]
  );

  const startTime = new Date(data.metadata.startTime * 1000);
  const endTime = new Date(data.metadata.endTime * 1000);

  return (
    <Grid sidebarVisible={sidebarVisible}>
      <Player>
        <RecordingPlayer
          clusterId={clusterId}
          sessionId={sessionId}
          durationMs={data.metadata.duration}
          recordingType="ssh"
          onToggleSidebar={toggleSidebar}
        />
      </Player>

      {sidebarVisible && (
        <Sidebar>
          <Flex
            flexDirection="column"
            gap={4}
            pt={3}
            minHeight={0}
            height="100%"
          >
            <Flex pl={3} pr={2} justifyContent="space-between">
              <BackLink to={cfg.getRecordingsRoute(clusterId)}>
                <ChevronLeft size="small" />
                Back to Session Recordings
              </BackLink>
            </Flex>

            <Flex alignItems="center" gap={3} px={3}>
              <Terminal />

              <H3>SSH Session</H3>
            </Flex>

            <InfoGrid>
              <InfoGridLabel>User</InfoGridLabel>

              <div>{data.metadata.user}</div>

              <InfoGridLabel>Resource</InfoGridLabel>

              <div>{data.metadata.resourceName}</div>

              <InfoGridLabel>Duration</InfoGridLabel>

              <div>
                {formatSessionRecordingDuration(data.metadata.duration)}
              </div>

              <InfoGridLabel>Cluster</InfoGridLabel>

              <div>{data.metadata.clusterName}</div>

              <InfoGridLabel>Start Time</InfoGridLabel>

              <div>{format(startTime, 'MMM dd, yyyy HH:mm')}</div>

              <InfoGridLabel>End Time</InfoGridLabel>

              <div>{format(endTime, 'MMM dd, yyyy HH:mm')}</div>
            </InfoGrid>

            {summary && <Summary>{summary}</Summary>}
          </Flex>
        </Sidebar>
      )}
    </Grid>
  );
}

const Grid = styled.div<{ sidebarVisible: boolean }>`
  display: grid;
  grid-template-areas: ${p =>
    p.sidebarVisible
      ? `'sidebar recording' 'timeline timeline'`
      : `'recording recording' 'timeline timeline'`};
  grid-template-columns: 1fr 4fr;
  grid-template-rows: 1fr auto;
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
`;

const InfoGrid = styled.div`
  display: grid;
  column-gap: ${p => p.theme.space[3]}px;
  row-gap: ${p => p.theme.space[2]}px;
  grid-template-columns: 80px 1fr;
  padding: 0 ${p => p.theme.space[3]}px;
`;

const Player = styled.div`
  grid-area: recording;
  display: flex;
  justify-content: center;
  align-items: center;
  position: relative;
`;

const Sidebar = styled.div`
  grid-area: sidebar;
  border-right: 1px solid ${p => p.theme.colors.spotBackground[1]};
  overflow: hidden;
  display: flex;
  flex-direction: column;
`;

const Summary = styled.div`
  border-top: 1px solid ${p => p.theme.colors.spotBackground[1]};
  overflow-y: auto;
  height: 100%;
  flex: 1;
  min-height: 0;
  padding: ${p => p.theme.space[3]}px ${p => p.theme.space[3]}px 0;
`;

const InfoGridLabel = styled.div`
  font-weight: bold;
  color: ${p => p.theme.colors.text.slightlyMuted};
`;

const BackLink = styled(Link)`
  color: ${p => p.theme.colors.text.slightlyMuted};
  text-decoration: none;
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: ${p => p.theme.space[2]}px;
`;
