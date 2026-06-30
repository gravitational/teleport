import type { QueryObserverResult } from '@tanstack/react-query';
import { useMemo } from 'react';
import { ErrorBoundary } from 'react-error-boundary';

import Flex from 'design/Flex';
import { Warning } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';

import cfg from 'e-teleport/config';
import { useSuspenseGetRecordingSummary } from 'e-teleport/services/recordings/hooks';
import { RECORDING_TYPES_WITH_SUMMARIES } from 'e-teleport/services/recordings/recordings';
import {
  NeedsFurtherReview,
  RecordingSummaryState,
  type SessionRecordingSummary,
} from 'e-teleport/services/recordings/types';
import { RiskLevel } from 'e-teleport/SessionRecordings/summary/RiskLevel';
import { StyledBadge } from 'e-teleport/SessionRecordings/summary/TimelineItem';
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
  summary: rawSummary,
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

  const summary =
    rawSummary?.state === RecordingSummaryState.Success
      ? (rawSummary.enhancedSummary ?? null)
      : null;

  const events = useMemo(() => {
    if (!summary) {
      return data.metadata.events;
    }

    const riskyEvents = summary.sessionEvents
      .filter(e =>
        [
          RiskLevelValue.Medium,
          RiskLevelValue.High,
          RiskLevelValue.Critical,
        ].includes(e.riskLevel)
      )
      .map(e => {
        if (e.inferenceErrorMessage) {
          return {
            type: SessionRecordingEventType.Risk,
            description: e.commandEventDetails
              ? 'Error analyzing command, review it further'
              : 'Error analyzing event, review it further',
            riskLevel: e.riskLevel,
            startTime: e.startOffset,
            endTime: e.endOffset,
            isError: true,
          } as SessionRecordingRiskEvent;
        }

        return {
          type: SessionRecordingEventType.Risk,
          description: e.timelineTitle,
          riskLevel: e.riskLevel,
          startTime: e.startOffset,
          endTime: e.endOffset,
        } as SessionRecordingRiskEvent;
      });

    return [...(data.metadata?.events || []), ...riskyEvents];
  }, [data.metadata, summary]);

  return (
    <SessionRecordingGrid
      hasSummary={!!rawSummary}
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
            {summary && (
              <>
                <InfoGridLabel>Risk Score</InfoGridLabel>

                <Flex alignItems="center" flexWrap="wrap" gap={2}>
                  <RiskLevel
                    riskLevel={summary.riskLevel}
                    data-testid="session-risk-level"
                  />

                  {summary.needsFurtherReviewReasons.length > 0 && (
                    <HoverTooltip
                      tipContent={getNeedsFurtherReviewTooltipContent(
                        summary.needsFurtherReviewReasons
                      )}
                    >
                      <StyledBadge
                        Icon={Warning}
                        label="Needs further review"
                        bordered
                      />
                    </HoverTooltip>
                  )}
                </Flex>
              </>
            )}
          </SessionRecordingDetails>

          {rawSummary && (
            <SessionSummary
              onPlay={handleTimelineTimeChange}
              metadata={data.metadata}
              data={rawSummary}
              onRefetch={onRefetchSummary}
              refetching={refetchingSummary}
            />
          )}

          <SidebarResizeHandle
            width={sidebarWidth}
            onChange={setSidebarWidth}
            defaultWidth={
              rawSummary
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

function getNeedsFurtherReviewReasonText(reason: NeedsFurtherReview): string {
  switch (reason) {
    case NeedsFurtherReview.TooLarge:
      return 'The recording was only partially analyzed due to its large size.';
    case NeedsFurtherReview.CommandAnalysisFailed:
      return 'One or more commands in this recording could not be analyzed.';
    case NeedsFurtherReview.FailedToFetchAccessRequest:
      return 'The access request associated with this session could not be fetched.';
    case NeedsFurtherReview.AccessRequestResourceMismatch:
      return 'The access request for this session does not reference the resource being connected to.';
    default:
      return 'This recording needs further review.';
  }
}

function getNeedsFurtherReviewTooltipContent(
  reasons: NeedsFurtherReview[]
): string {
  if (reasons.length === 0) {
    return 'This recording needs further review.';
  }
  if (reasons.length === 1) {
    return `${getNeedsFurtherReviewReasonText(reasons[0])} The recording needs further review.`;
  }
  return (
    'This recording needs further review for the following reasons: ' +
    reasons.map(getNeedsFurtherReviewReasonText).join(' ')
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
