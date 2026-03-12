import {
  arrow,
  autoUpdate,
  flip,
  offset,
  shift,
  size,
  useDismiss,
  useFloating,
  useInteractions,
} from '@floating-ui/react';
import {
  format,
  formatDistanceToNow,
  formatDuration,
  intervalToDuration,
  isValid,
  type Duration,
} from 'date-fns';
import { useCallback, useState, type MouseEvent, type ReactNode } from 'react';
import { Link } from 'react-router';
import styled, { keyframes } from 'styled-components';

import Box from 'design/Box';
import { Button, ButtonBorder, ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import { ArrowRight, ChevronRight, Sparkle, Spinner, User } from 'design/Icon';
import Modal from 'design/Modal';
import { StyledPopover } from 'design/Popover';
import Text from 'design/Text';
import { HoverTooltip } from 'design/Tooltip';
import { Markdown } from 'shared/components/Markdown/Markdown';

import { useGetRecordingSummary } from 'e-teleport/services/recordings/hooks';
import { RECORDING_TYPES_WITH_SUMMARIES } from 'e-teleport/services/recordings/recordings';
import {
  RecordingSummaryState,
  type SessionRecordingSummary,
} from 'e-teleport/services/recordings/types';
import { SessionRecordingTimeline } from 'e-teleport/SessionRecordings/summary/SessionRecordingTimeline';
import {
  MarkdownContainer,
  SummaryInfo,
} from 'e-teleport/SessionRecordings/summary/SessionSummary';
import cfg from 'teleport/config';
import {
  formatSessionRecordingDuration,
  getRecordingTypeInfo,
  type RecordingActionProps,
} from 'teleport/SessionRecordings/list/RecordingItem';
import useStickyClusterId from 'teleport/useStickyClusterId';

const Arrow = styled.div<{ placement: string }>`
  position: absolute;
  width: 8px;
  height: 8px;
  background: ${p => p.theme.colors.levels.surface};
  transform: rotate(45deg);
  z-index: -1;

  ${p =>
    p.placement.startsWith('left')
      ? `
    right: -4px;
    border-right: 1px solid ${p.theme.colors.spotBackground[1]};
    border-top: 1px solid ${p.theme.colors.spotBackground[1]};
  `
      : `
    left: -4px;
    border-left: 1px solid ${p.theme.colors.spotBackground[1]};
    border-bottom: 1px solid ${p.theme.colors.spotBackground[1]};
  `}
`;

export function ViewSummary(props: RecordingActionProps) {
  const [open, setOpen] = useState(false);
  const [arrowEl, setArrowEl] = useState<HTMLDivElement>(null);
  const { clusterId } = useStickyClusterId();

  const { data, error, isError, isFetching, isRefetching, refetch } =
    useGetRecordingSummary(
      {
        clusterId,
        sessionId: props.sessionId,
      },
      { enabled: false }
    );

  const handleClick = useCallback(
    async (event: MouseEvent<HTMLButtonElement>) => {
      event.preventDefault();
      event.stopPropagation();

      try {
        await refetch();
      } catch {
        // Ignore errors here; they are handled in the UI.
      }

      setOpen(true);
    },
    [refetch]
  );

  const { context, floatingStyles, middlewareData, refs } = useFloating({
    open,
    onOpenChange: setOpen,
    middleware: [
      offset(8),
      flip({
        mainAxis: true,
        crossAxis: false,
        fallbackAxisSideDirection: 'none',
      }),
      shift({ padding: 8 }),
      size({
        apply({ availableWidth, availableHeight, elements }) {
          elements.floating.style.maxWidth = `${Math.min(900, availableWidth)}px`;
          elements.floating.style.maxHeight = `${Math.min(
            700,
            availableHeight
          )}px`;
        },
        padding: 8,
      }),
      arrow({
        element: arrowEl,
        padding: 8,
      }),
    ],
    placement: 'left',
    whileElementsMounted: autoUpdate,
  });

  const dismiss = useDismiss(context);

  const { getReferenceProps, getFloatingProps } = useInteractions([dismiss]);

  if (!RECORDING_TYPES_WITH_SUMMARIES.includes(props.recordingType)) {
    return null;
  }

  return (
    <>
      <HoverTooltip
        tipContent={
          isFetching ? 'Loading session summary…' : 'View session summary'
        }
        disabled={open || isFetching}
      >
        <ButtonBorder
          intent={open ? 'primary' : 'neutral'}
          width="32px"
          padding="0"
          aria-label="View session summary"
          onClick={handleClick}
          ref={refs.setReference}
          {...getReferenceProps()}
        >
          {isFetching ? (
            <RotatingSpinner size="small" />
          ) : (
            <Sparkle size="small" />
          )}
        </ButtonBorder>
      </HoverTooltip>

      {open && (data || isError) && (
        <Modal open={true} BackdropProps={{ invisible: true }}>
          <MoreRoundedPopover
            shadow={true}
            ref={refs.setFloating}
            style={{ ...floatingStyles, overflow: 'visible' }}
            {...getFloatingProps()}
          >
            <Arrow
              ref={setArrowEl}
              placement={context.placement}
              style={{
                left: middlewareData.arrow?.x ?? '',
                top: middlewareData.arrow?.y ?? '',
              }}
            />

            {isError ? (
              <SessionSummaryContainer>
                <Text color="error.main" fontWeight="bold">
                  Error loading session summary
                </Text>

                <Text>{error.message}</Text>

                <ButtonSecondary onClick={() => refetch()} mt={1}>
                  Retry
                </ButtonSecondary>
              </SessionSummaryContainer>
            ) : (
              <SessionSummaryContent
                data={data}
                isRefetching={isRefetching}
                onRefetch={refetch}
                {...props}
              />
            )}
          </MoreRoundedPopover>
        </Modal>
      )}
    </>
  );
}

const MoreRoundedPopover = styled(StyledPopover)`
  background: ${p => p.theme.colors.levels.surface};
  border-radius: ${p => p.theme.radii[3]}px;
`;

interface SessionSummaryContentProps extends RecordingActionProps {
  data: SessionRecordingSummary;
  isRefetching: boolean;
  onRefetch: () => void;
}

function SessionSummaryContent({
  createdDate,
  data,
  durationMs,
  isRefetching,
  onRefetch,
  recordingType,
  sessionId,
  hostname,
  username,
}: SessionSummaryContentProps) {
  const { clusterId } = useStickyClusterId();

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

        <ButtonSecondary onClick={onRefetch} disabled={isRefetching} mt={1}>
          {isRefetching ? 'Reloading...' : 'Reload'}
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
      const { icon: Icon, label } = getRecordingTypeInfo(recordingType);

      const url = cfg.getPlayerRoute(
        { clusterId, sid: sessionId },
        {
          recordingType,
          durationMs,
        }
      );

      return (
        <Flex
          borderRadius={3}
          flexDirection="column"
          width="900px"
          maxHeight="700px"
        >
          <Flex
            alignItems="center"
            justifyContent="space-between"
            borderBottom="1px solid"
            borderColor="spotBackground.1"
            pr={2}
            pl={3}
            py={2}
          >
            <Box>
              <Flex gap={2}>
                <Icon size="small" />

                <Text fontWeight="500" mr={2}>
                  {label}
                </Text>

                <ItemSpan>
                  <User size="small" color="sessionRecording.user" />

                  <Text>{username}</Text>
                </ItemSpan>

                <ArrowRight size="small" color="text.slightlyMuted" />

                <ItemSpan>
                  <Icon size="small" color="sessionRecording.resource" />

                  <Text>{hostname}</Text>
                </ItemSpan>

                <Text
                  color="text.slightlyMuted"
                  fontSize="small"
                  fontWeight="400"
                >
                  {format(createdDate, 'MMM dd, yyyy HH:mm')} for{' '}
                  <strong>{formatSessionRecordingDuration(durationMs)}</strong>
                </Text>
              </Flex>
            </Box>

            <Button
              as={Link}
              to={url}
              compact
              fill="minimal"
              intent="neutral"
              pr={1}
              pl={2}
            >
              View session recording <ChevronRight size="small" ml={1} />
            </Button>
          </Flex>

          <Flex minHeight={0} height="100%">
            <Box
              flex="2"
              borderRight="1px solid"
              borderColor="spotBackground.1"
              pr={4}
              px={1}
              py={2}
              style={{ overflowY: 'auto' }}
              data-scrollbar="default"
            >
              <SessionRecordingTimeline
                commands={data.enhancedSummary.commands}
                onPlay={null}
                sessionDuration={durationMs}
                inferenceDuration={duration}
              />
            </Box>

            <Box
              flex="3"
              px={3}
              py={3}
              style={{ overflowY: 'auto' }}
              data-scrollbar="default"
            >
              <MarkdownContainer>
                <Markdown text={data.enhancedSummary.detailedDescription} />
              </MarkdownContainer>
            </Box>
          </Flex>
        </Flex>
      );
    }

    return (
      <Box
        width="500px"
        p={3}
        style={{ overflowY: 'auto' }}
        data-scrollbar="default"
      >
        <MarkdownContainer>
          <Markdown text={data.content} />

          <SummaryInfo>
            <strong>AI can make mistakes.</strong> {content}
          </SummaryInfo>
        </MarkdownContainer>
      </Box>
    );
  }

  return null;
}

const SessionSummaryContainer = styled(Flex)`
  align-items: center;
  flex-direction: column;
  justify-content: center;
  min-width: 400px;
  max-width: 600px;
  min-height: 150px;
  gap: ${p => p.theme.space[2]}px;
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px;
`;

const ItemSpan = styled.span`
  background: ${p => p.theme.colors.spotBackground[0]};
  line-height: 1;
  padding: ${p => p.theme.space[1]}px ${p => p.theme.space[1]}px;
  border-radius: ${p => p.theme.radii[3]}px;
  display: inline-flex;
  align-items: center;
  font-size: 13px;
  gap: ${p => p.theme.space[1]}px;
`;

const rotate = keyframes`
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
`;

const RotatingSpinner = styled(Spinner)`
  animation: ${rotate} 1s linear infinite;
`;
