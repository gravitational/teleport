import type { QueryObserverResult } from '@tanstack/react-query';
import { useMemo } from 'react';
import { ErrorBoundary } from 'react-error-boundary';

import cfg from 'e-teleport/config';
import { useSuspenseGetRecordingSummary } from 'e-teleport/services/recordings/hooks';
import { RECORDING_TYPES_WITH_SUMMARIES } from 'e-teleport/services/recordings/recordings';
import {
  RecordingSummaryState,
  type SessionRecordingSummary,
} from 'e-teleport/services/recordings/types';
import { RiskLevel } from 'e-teleport/SessionRecordings/summary/RiskLevel';
import {
  SessionRecordingEventType,
  type SessionRecordingRiskEvent,
} from 'teleport/services/recordings';
import { useSuspenseGetRecordingMetadata } from 'teleport/services/recordings/hooks';
import { RiskLevel as RiskLevelValue } from 'teleport/services/recordings/types';
import { RecordingPlayer } from 'teleport/SessionRecordings/view/RecordingPlayer';
import {
  PlayerContainer,
  SessionRecordingGrid,
  SidebarContainer,
  TimelineContainer,
  type RecordingWithMetadataProps,
} from 'teleport/SessionRecordings/view/RecordingWithMetadata';
import {
  RecordingWithSummaryPlayer,
  type RecordingWithSummaryProps,
} from 'teleport/SessionRecordings/view/RecordingWithSummary';
import {
  InfoGridLabel,
  SessionRecordingDetails,
} from 'teleport/SessionRecordings/view/SessionRecordingDetails';
import {
  DEFAULT_SIDEBAR_WIDTH,
  SidebarResizeHandle,
} from 'teleport/SessionRecordings/view/SidebarResizeHandle';
import { RecordingTimeline } from 'teleport/SessionRecordings/view/Timeline/RecordingTimeline';
import { useRecording } from 'teleport/SessionRecordings/view/useRecording';
import { ViewSessionRecordingRoute } from 'teleport/SessionRecordings/view/ViewSessionRecordingRoute';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { SessionSummary } from '../summary/SessionSummary';

export function ViewSessionRecordingRouteE() {
  return (
    <ViewSessionRecordingRoute
      withMetadataComponent={RecordingWithMetadataWrapper}
      withSummaryComponent={RecordingWithSummaryE}
    />
  );
}

function RecordingWithMetadataWrapper(props: RecordingWithMetadataProps) {
  if (
    cfg.oss.sessionSummarizerEnabled &&
    RECORDING_TYPES_WITH_SUMMARIES.includes(props.recordingType)
  ) {
    return (
      <ErrorBoundary
        fallback={
          <RecordingWithMetadataE
            {...props}
            summary={null}
            onRefetchSummary={null}
            refetchingSummary={false}
          />
        }
      >
        <RecordingWithMetadataLoadSummary {...props} />
      </ErrorBoundary>
    );
  }

  return (
    <RecordingWithMetadataE
      {...props}
      summary={null}
      onRefetchSummary={null}
      refetchingSummary={false}
    />
  );
}

function RecordingWithMetadataLoadSummary(props: RecordingWithMetadataProps) {
  const { clusterId } = useStickyClusterId();

  const { data, isRefetching, refetch } = useSuspenseGetRecordingSummary({
    clusterId,
    sessionId: props.sessionId,
  });

  return (
    <RecordingWithMetadataE
      {...props}
      summary={data}
      onRefetchSummary={refetch}
      refetchingSummary={isRefetching}
    />
  );
}

const DEFAULT_SIDEBAR_WIDTH_WITH_SUMMARY = 400;

interface RecordingWithMetadataEProps extends RecordingWithMetadataProps {
  summary: SessionRecordingSummary | null;
  onRefetchSummary: () => Promise<
    QueryObserverResult<SessionRecordingSummary>
  > | null;
  refetchingSummary: boolean;
}

function RecordingWithMetadataE({
  clusterId,
  sessionId,
  onRefetchSummary,
  refetchingSummary,
  summary,
}: RecordingWithMetadataEProps) {
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

  const events = useMemo(() => {
    if (
      summary?.state !== RecordingSummaryState.Success ||
      !summary.enhancedSummary
    ) {
      return data.metadata.events;
    }

    for (const [index, command] of summary.enhancedSummary.commands.entries()) {
      switch (index) {
        case 1:
          {
            command.startOffset = 4000;
            command.endOffset = 8000;
          }
          break;
        case 2:
          {
            command.startOffset = 8000;
            command.endOffset = 12000;
          }
          break;
        case 3:
          {
            command.startOffset = 13000;
            command.endOffset = 14000;
          }
          break;
        case 4:
          {
            command.startOffset = 18000;
            command.endOffset = 21000;
          }
          break;
        case 5:
          {
            command.startOffset = 28000;
            command.endOffset = 18000;
          }
          break;
      }
    }

    const riskyCommandEvents = summary.enhancedSummary.commands
      .filter(cmd =>
        [
          RiskLevelValue.Medium,
          RiskLevelValue.High,
          RiskLevelValue.Critical,
        ].includes(cmd.riskLevel)
      )
      .map(
        cmd =>
          ({
            type: SessionRecordingEventType.Risk,
            description: cmd.timelineTitle,
            riskLevel: cmd.riskLevel,
            startTime: cmd.startOffset,
            endTime: cmd.endOffset,
          }) as SessionRecordingRiskEvent
      );

    return [...(data.metadata?.events || []), ...riskyCommandEvents];
  }, [data.metadata, summary]);

  return (
    <SessionRecordingGrid
      hasSummary={!!summary}
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
          >
            {summary?.state === RecordingSummaryState.Success &&
              summary.enhancedSummary && (
                <>
                  <InfoGridLabel>Risk Score</InfoGridLabel>

                  <div>
                    <RiskLevel riskLevel={summary.enhancedSummary.riskLevel} />
                  </div>
                </>
              )}
          </SessionRecordingDetails>

          {summary && (
            <SessionSummary
              onPlay={handleTimelineTimeChange}
              metadata={data.metadata}
              data={summary}
              onRefetch={onRefetchSummary}
              refetching={refetchingSummary}
            />
          )}

          <SidebarResizeHandle
            width={sidebarWidth}
            onChange={setSidebarWidth}
            defaultWidth={
              summary
                ? DEFAULT_SIDEBAR_WIDTH_WITH_SUMMARY
                : DEFAULT_SIDEBAR_WIDTH
            }
          />
        </SidebarContainer>
      )}

      {data.frames.length > 0 && !timelineHidden && (
        <TimelineContainer>
          <RecordingTimeline
            duration={data.metadata.duration}
            startTime={data.metadata.startTime}
            events={events}
            frames={data.frames}
            onTimeChange={handleTimelineTimeChange}
            ref={timelineRef}
            showAbsoluteTime={false} // TODO(ryan): add with the keyboard shortcuts PR
          />
        </TimelineContainer>
      )}
    </SessionRecordingGrid>
  );
}

function RecordingWithSummaryE({
  clusterId,
  sessionId,
  durationMs,
  recordingType,
}: RecordingWithSummaryProps) {
  const { data, isRefetching, refetch } = useSuspenseGetRecordingSummary({
    clusterId,
    sessionId,
  });

  const { containerRef, playerRef, sidebarHidden, goToTime, toggleSidebar } =
    useRecording();

  return (
    <SessionRecordingGrid sidebarHidden={sidebarHidden} ref={containerRef}>
      <PlayerContainer>
        <RecordingWithSummaryPlayer
          clusterId={clusterId}
          sessionId={sessionId}
          durationMs={durationMs}
          recordingType={recordingType}
          playerRef={playerRef}
          toggleSidebar={toggleSidebar}
        />
      </PlayerContainer>

      {!sidebarHidden && (
        <SidebarContainer>
          <SessionRecordingDetails
            sessionId={sessionId}
            recordingType={recordingType}
            metadata={null}
          />

          <SessionSummary
            data={data}
            metadata={null}
            onPlay={goToTime}
            onRefetch={refetch}
            refetching={isRefetching}
          />
        </SidebarContainer>
      )}
    </SessionRecordingGrid>
  );
}
