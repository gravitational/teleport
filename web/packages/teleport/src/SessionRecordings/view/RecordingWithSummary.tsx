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

import { useCallback, useMemo } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';

import Flex from 'design/Flex';
import { ChevronLeft } from 'design/Icon';
import { H3 } from 'design/Text';
import { ErrorSuspenseWrapper } from 'shared/components/ErrorSuspenseWrapper/ErrorSuspenseWrapper';
import { useLocalStorage } from 'shared/hooks/useLocalStorage';

import cfg from 'teleport/config';
import { type RecordingType } from 'teleport/services/recordings';
import { VALID_RECORDING_TYPES } from 'teleport/services/recordings/recordings';
import { KeysEnum } from 'teleport/services/storageService';
import { getRecordingTypeInfo } from 'teleport/SessionRecordings/list/RecordingItem';
import { RecordingPlayer } from 'teleport/SessionRecordings/view/RecordingPlayer';
import type { SummarySlot } from 'teleport/SessionRecordings/view/RecordingWithMetadata';
import {
  RecordingPlayerError,
  RecordingPlayerLoading,
  RecordingPlayerWithLoadDuration,
} from 'teleport/SessionRecordings/view/ViewSessionRecordingRoute';

interface RecordingWithSummaryProps {
  clusterId: string;
  durationMs: number;
  sessionId: string;
  recordingType: RecordingType;
  summarySlot: SummarySlot;
}

export function RecordingWithSummary({
  clusterId,
  durationMs,
  recordingType,
  sessionId,
  summarySlot,
}: RecordingWithSummaryProps) {
  const [sidebarHidden, setSidebarHidden] = useLocalStorage(
    KeysEnum.SESSION_RECORDING_SIDEBAR_HIDDEN,
    false
  );

  const summary = useMemo(
    () => summarySlot(sessionId, recordingType),
    [summarySlot, sessionId, recordingType]
  );

  const toggleSidebar = useCallback(() => {
    // setSidebarHidden(prev => !prev) does not work with useLocalStorage, it stops working after the first toggle
    setSidebarHidden(!sidebarHidden);
  }, [sidebarHidden, setSidebarHidden]);

  const { icon: Icon, label } = getRecordingTypeInfo(recordingType);

  const validRecordingType = VALID_RECORDING_TYPES.includes(recordingType);
  const validDuration = Number.isInteger(durationMs) && durationMs > 0;

  const shouldFetchSessionDuration = !validRecordingType || !validDuration;

  let player = (
    <RecordingPlayer
      clusterId={clusterId}
      sessionId={sessionId}
      durationMs={durationMs}
      recordingType={recordingType}
      onToggleSidebar={toggleSidebar}
    />
  );

  if (shouldFetchSessionDuration) {
    player = (
      <ErrorSuspenseWrapper
        errorComponent={RecordingPlayerError}
        loadingComponent={RecordingPlayerLoading}
      >
        <RecordingPlayerWithLoadDuration
          clusterId={clusterId}
          sessionId={sessionId}
          onToggleSidebar={toggleSidebar}
        />
      </ErrorSuspenseWrapper>
    );
  }

  if (!summary) {
    return player;
  }

  return (
    <Grid sidebarHidden={sidebarHidden}>
      <Player>{player}</Player>

      {!sidebarHidden && (
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
              <Icon size="small" />

              <H3>{label}</H3>
            </Flex>

            <Summary>{summary}</Summary>
          </Flex>
        </Sidebar>
      )}
    </Grid>
  );
}

const Grid = styled.div<{ sidebarHidden: boolean }>`
  background: ${p => p.theme.colors.levels.sunken};
  display: grid;
  grid-template-areas: ${p =>
    p.sidebarHidden ? `'recording recording'` : `'sidebar recording'`};
  grid-template-columns: 1fr 4fr;
  grid-template-rows: 1fr auto;
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
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

const BackLink = styled(Link)`
  color: ${p => p.theme.colors.text.slightlyMuted};
  text-decoration: none;
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: ${p => p.theme.space[2]}px;
`;
