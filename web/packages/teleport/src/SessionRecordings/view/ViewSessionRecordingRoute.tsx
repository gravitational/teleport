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

import { Suspense, useEffect, type ComponentType } from 'react';
import { ErrorBoundary } from 'react-error-boundary';

import { Danger } from 'design/Alert';
import Box from 'design/Box';
import Flex from 'design/Flex';
import { Indicator } from 'design/Indicator';
import { ErrorSuspenseWrapper } from 'shared/components/ErrorSuspenseWrapper/ErrorSuspenseWrapper';

import { useLocation, useParams } from 'teleport/components/Router';
import { UrlPlayerParams } from 'teleport/config';
import { getUrlParameter } from 'teleport/services/history';
import { RecordingType } from 'teleport/services/recordings';
import {
  useSuspenseGetRecordingDuration,
  useSuspenseGetRecordingMetadata,
} from 'teleport/services/recordings/hooks';
import {
  RECORDING_TYPES_WITH_METADATA,
  VALID_RECORDING_TYPES,
} from 'teleport/services/recordings/recordings';
import type { RecordingWithSummaryProps } from 'teleport/SessionRecordings/view/RecordingWithSummary';
import { SessionRecordingDetails } from 'teleport/SessionRecordings/view/SessionRecordingDetails';
import { RecordingTimeline } from 'teleport/SessionRecordings/view/Timeline/RecordingTimeline';
import { useRecording } from 'teleport/SessionRecordings/view/useRecording';

import { RecordingPlayer } from './RecordingPlayer';
import {
  PlayerContainer,
  SessionRecordingGrid,
  SidebarContainer,
  TimelineContainer,
  type RecordingWithMetadataProps,
} from './RecordingWithMetadata';
import { SidebarResizeHandle } from './SidebarResizeHandle';

interface ViewSessionRecordingRouteProps {
  withMetadataComponent?: ComponentType<RecordingWithMetadataProps>;
  withSummaryComponent?: ComponentType<RecordingWithSummaryProps>;
}

export function ViewSessionRecordingRoute({
  withMetadataComponent: RecordingWithMetadataComponent = RecordingWithMetadata,
  withSummaryComponent: RecordingWithSummaryComponent,
}: ViewSessionRecordingRouteProps) {
  const { sid, clusterId } = useParams<UrlPlayerParams>();
  const { search } = useLocation();

  const recordingType = getUrlParameter(
    'recordingType',
    search
  ) as RecordingType;

  useEffect(() => {
    document.title = `Play ${sid} • ${clusterId}`;
  }, [sid, clusterId]);

  const validRecordingType = VALID_RECORDING_TYPES.includes(recordingType);
  const durationMs = Number(getUrlParameter('durationMs', search));
  const validDuration = Number.isInteger(durationMs) && durationMs > 0;

  const shouldFetchSessionDuration = !validRecordingType || !validDuration;

  let player = (
    <RecordingPlayer
      clusterId={clusterId}
      sessionId={sid}
      durationMs={durationMs}
      recordingType={recordingType}
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
          sessionId={sid}
        />
      </ErrorSuspenseWrapper>
    );
  }

  const recordingWithSummaryComponent = RecordingWithSummaryComponent ? (
    <RecordingWithSummaryComponent
      clusterId={clusterId}
      sessionId={sid}
      durationMs={durationMs}
      recordingType={recordingType}
    />
  ) : null;

  if (RECORDING_TYPES_WITH_METADATA.includes(recordingType)) {
    // If the recording type is SSH, try to load the session metadata (ViewTerminalRecording)
    // and render the SSH player with the session metadata/summary.
    // If that errors (such as during a proxy upgrade), we fall back to the
    // RecordingPlayerWrapper which will fetch the session duration and render the player.
    // This is to ensure that the player can still be rendered even if the session metadata
    // cannot be fetched, allowing users to still view the recording.

    const recordingWithMetadataComponent = (
      <RecordingWithMetadataComponent
        clusterId={clusterId}
        recordingType={recordingType}
        sessionId={sid}
      />
    );

    if (recordingWithSummaryComponent) {
      return (
        <Suspense fallback={<RecordingPlayerLoading />}>
          <ErrorBoundary fallback={player}>
            <ErrorBoundary fallback={recordingWithSummaryComponent}>
              {recordingWithMetadataComponent}
            </ErrorBoundary>
          </ErrorBoundary>
        </Suspense>
      );
    }

    return (
      <Suspense fallback={<RecordingPlayerLoading />}>
        <ErrorBoundary fallback={player}>
          {recordingWithMetadataComponent}
        </ErrorBoundary>
      </Suspense>
    );
  }

  if (recordingWithSummaryComponent) {
    return (
      <Suspense fallback={<RecordingPlayerLoading />}>
        <ErrorBoundary fallback={player}>
          {recordingWithSummaryComponent}
        </ErrorBoundary>
      </Suspense>
    );
  }

  return player;
}

function RecordingWithMetadata({
  clusterId,
  sessionId,
}: RecordingWithMetadataProps) {
  const { data } = useSuspenseGetRecordingMetadata({
    clusterId,
    sessionId,
  });

  const {
    containerRef,
    playerRef,
    timelineRef,
    fullscreen,
    sidebarHidden,
    sidebarWidth,
    setSidebarWidth,
    timelineHidden,
    handleTimeChange,
    handleTimelineTimeChange,
    toggleSidebar,
    toggleTimeline,
    handleToggleFullscreen,
  } = useRecording();

  return (
    <SessionRecordingGrid
      sidebarHidden={sidebarHidden}
      sidebarWidth={sidebarWidth}
      ref={containerRef}
    >
      <PlayerContainer>
        <RecordingPlayer
          clusterId={clusterId}
          sessionId={sessionId}
          durationMs={data.metadata.duration}
          recordingType={data.metadata.type}
          onToggleFullscreen={handleToggleFullscreen}
          fullscreen={fullscreen.active}
          onToggleSidebar={toggleSidebar}
          onToggleTimeline={toggleTimeline}
          onTimeChange={handleTimeChange}
          initialCols={data.metadata.startCols}
          initialRows={data.metadata.startRows}
          events={data.metadata.events}
          ref={playerRef}
        />
      </PlayerContainer>

      {!sidebarHidden && (
        <SidebarContainer>
          <SessionRecordingDetails
            sessionId={sessionId}
            recordingType={data.metadata.type}
            metadata={data.metadata}
          />
          <SidebarResizeHandle
            width={sidebarWidth}
            onChange={setSidebarWidth}
          />
        </SidebarContainer>
      )}

      {data.frames.length > 0 && !timelineHidden && (
        <TimelineContainer>
          <RecordingTimeline
            duration={data.metadata.duration}
            events={data.metadata.events}
            frames={data.frames}
            startTime={data.metadata.startTime}
            onTimeChange={handleTimelineTimeChange}
            ref={timelineRef}
            showAbsoluteTime={false} // TODO(ryan): add with the keyboard shortcuts PR
          />
        </TimelineContainer>
      )}
    </SessionRecordingGrid>
  );
}

export function RecordingPlayerLoading() {
  return (
    <Flex width="100%" height="100%" flexDirection="column">
      <Box textAlign="center" mx={10} mt={5}>
        <Indicator />
      </Box>
    </Flex>
  );
}

export function RecordingPlayerError() {
  return (
    <Flex width="100%" height="100%" flexDirection="column">
      <Box textAlign="center" mx={10} mt={5}>
        <Danger mb={0}>
          Unable to determine the length of this session. The session recording
          may be incomplete or corrupted.
        </Danger>
      </Box>
    </Flex>
  );
}

interface RecordingPlayerWithLoadDurationProps {
  clusterId: string;
  sessionId: string;
  onToggleSidebar?: () => void;
}

export function RecordingPlayerWithLoadDuration({
  clusterId,
  sessionId,
  onToggleSidebar,
}: RecordingPlayerWithLoadDurationProps) {
  const { data } = useSuspenseGetRecordingDuration({
    clusterId,
    sessionId,
  });

  return (
    <RecordingPlayer
      clusterId={clusterId}
      sessionId={sessionId}
      durationMs={data.durationMs}
      recordingType={data.recordingType}
      onToggleSidebar={onToggleSidebar}
    />
  );
}
