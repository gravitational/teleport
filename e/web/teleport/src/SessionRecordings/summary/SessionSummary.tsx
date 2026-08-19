import {
  formatDistanceToNow,
  formatDuration,
  intervalToDuration,
  isValid,
  type Duration,
} from 'date-fns';
import { type ReactNode } from 'react';
import styled from 'styled-components';

import Box from 'design/Box';
import { ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import Text, { H3 } from 'design/Text';
import { Markdown } from 'shared/components/Markdown/Markdown';

import {
  RecordingSummaryState,
  type SessionRecordingSummary,
} from 'e-teleport/services/recordings/types';
import { SessionDescription } from 'e-teleport/SessionRecordings/summary/SessionDescription';
import { SessionRecordingTimeline } from 'e-teleport/SessionRecordings/summary/SessionRecordingTimeline';
import type { SessionRecordingMetadata } from 'teleport/services/recordings';

interface SessionSummaryInnerProps {
  onPlay: (timestamp: number) => void;
  metadata: SessionRecordingMetadata;
  onRefetch: () => void;
  refetching: boolean;
  data: SessionRecordingSummary;
}

export function SessionSummary({
  onPlay,
  metadata,
  data,
  refetching,
  onRefetch,
}: SessionSummaryInnerProps) {
  if (data.state === RecordingSummaryState.Pending) {
    const inferenceStartedAt = new Date(data.inferenceStartedAt);

    let content: ReactNode | null = null;
    if (isValid(inferenceStartedAt)) {
      content = (
        <Text color="text.slightlyMuted">
          Started summarizing{' '}
          {formatDistanceToNow(inferenceStartedAt, { addSuffix: true })}
        </Text>
      );
    }

    return (
      <SessionSummaryContainer>
        <Text fontWeight="bold">
          Session summary is currently being generated
        </Text>

        {content}

        <ButtonSecondary onClick={onRefetch} disabled={refetching} mt={1}>
          {refetching ? 'Reloading...' : 'Reload'}
        </ButtonSecondary>
      </SessionSummaryContainer>
    );
  }

  if (data.state === RecordingSummaryState.Error) {
    if (
      data.errorMessage.includes('session transcript exceeds maximum length')
    ) {
      return (
        <SessionSummaryContainer>
          <Text color="text.slightlyMuted">
            This session was not summarized because it is too large.
          </Text>
        </SessionSummaryContainer>
      );
    }

    return (
      <SessionSummaryContainer>
        <Text color="error.main" fontWeight="bold">
          There was an error generating the session summary
        </Text>

        <Text>{data.errorMessage}</Text>
      </SessionSummaryContainer>
    );
  }

  if (data.state === RecordingSummaryState.NoInferencePolicy) {
    return (
      <SessionSummaryContainer>
        <Text color="text.slightlyMuted">
          This session was not summarized because no inference policy applies to
          it.
        </Text>
      </SessionSummaryContainer>
    );
  }

  if (data.state === RecordingSummaryState.Success) {
    const inferenceStartedAt = new Date(data.inferenceStartedAt);
    const inferenceFinishedAt = new Date(data.inferenceFinishedAt);

    let duration: Duration | null = null;
    let content: ReactNode | null = null;
    if (isValid(inferenceStartedAt) && isValid(inferenceFinishedAt)) {
      duration = intervalToDuration({
        start: inferenceStartedAt,
        end: inferenceFinishedAt,
      });

      content = <>Summarization took {formatDuration(duration)}.</>;
    }

    if (data.enhancedSummary) {
      return (
        <EnhancedSummaryContainer data-testid="session-summary">
          <Box px="10px">
            <H3 ml="6px">Session Summary</H3>

            <SessionDescription
              shortDescription={data.enhancedSummary.shortDescription}
              detailedDescription={data.enhancedSummary.detailedDescription}
            />
          </Box>

          <SessionRecordingTimeline
            events={data.enhancedSummary.sessionEvents}
            onPlay={onPlay}
            sessionDuration={metadata?.duration}
            inferenceDuration={duration}
          />
        </EnhancedSummaryContainer>
      );
    }

    return (
      <MarkdownContainer>
        <Markdown text={data.content} />

        <SummaryInfo>
          <strong>AI can make mistakes.</strong> {content}
        </SummaryInfo>
      </MarkdownContainer>
    );
  }

  return null;
}

const SessionSummaryContainer = styled(Flex)`
  align-items: center;
  flex-direction: column;
  justify-content: center;
  width: 100%;
  gap: ${p => p.theme.space[2]}px;
`;

export const MarkdownContainer = styled.div`
  overflow-y: auto;
  min-height: 0;
  padding: ${p => p.theme.space[3]}px;
  border-top: 1px solid ${p => p.theme.colors.spotBackground[1]};

  p {
    margin: 0 0 ${p => p.theme.space[3]}px 0;
  }

  h1 {
    font-size: 20px;
    margin-top: 0;
  }

  h2 {
    font-size: 18px;
    margin: 0 0 ${p => p.theme.space[2]}px;
  }

  h3 {
    font-size: 16px;
    margin: 0 0 ${p => p.theme.space[2]}px;
  }
`;

const EnhancedSummaryContainer = styled.div`
  overflow-y: auto;
  border-top: 1px solid ${p => p.theme.colors.spotBackground[1]};
  padding-top: ${p => p.theme.space[3]}px;

  p {
    margin: 0;
  }
`;

export const SummaryInfo = styled(Box)`
  background: ${p => p.theme.colors.levels.elevated};
  border-radius: ${p => p.theme.radii[3]}px;
  color: ${p => p.theme.colors.text.slightlyMuted};
  font-size: 13px;
  padding: ${p => p.theme.space[2]}px;
  margin-bottom: ${p => p.theme.space[3]}px;
  line-height: 1.4;
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
`;
