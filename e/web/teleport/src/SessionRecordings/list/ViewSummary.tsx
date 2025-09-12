import {
  arrow,
  autoUpdate,
  offset,
  shift,
  size,
  useDismiss,
  useFloating,
  useInteractions,
} from '@floating-ui/react';
import {
  formatDistanceToNow,
  formatDuration,
  intervalToDuration,
  isValid,
} from 'date-fns';
import { useCallback, useState, type MouseEvent, type ReactNode } from 'react';
import type { FallbackProps } from 'react-error-boundary';
import styled from 'styled-components';

import Box from 'design/Box';
import { ButtonBorder, ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import { ChatCircleSparkle } from 'design/Icon';
import { Indicator } from 'design/Indicator';
import Modal from 'design/Modal';
import { StyledPopover } from 'design/Popover';
import Text, { H3 } from 'design/Text';
import { HoverTooltip } from 'design/Tooltip';
import { ErrorSuspenseWrapper } from 'shared/components/ErrorSuspenseWrapper/ErrorSuspenseWrapper';
import { Markdown } from 'shared/components/Markdown/Markdown';
import { getErrorMessage } from 'shared/utils/error';

import { useSuspenseGetRecordingSummary } from 'e-teleport/services/recordings/hooks';
import { RecordingSummaryState } from 'e-teleport/services/recordings/types';
import useStickyClusterId from 'teleport/useStickyClusterId';

interface ViewSummaryProps {
  sessionId: string;
}

const Arrow = styled.div<{ placement: string }>`
  position: absolute;
  width: 8px;
  height: 8px;
  background: ${p => p.theme.colors.levels.elevated};
  transform: rotate(45deg);
  z-index: -1;
  right: -4px;
  border-right: 1px solid ${p => p.theme.colors.spotBackground[1]};
  border-top: 1px solid ${p => p.theme.colors.spotBackground[1]};
`;

export function ViewSummary({ sessionId }: ViewSummaryProps) {
  const [open, setOpen] = useState(false);

  const [arrowEl, setArrowEl] = useState<HTMLDivElement>(null);

  const handleClick = useCallback((event: MouseEvent<HTMLButtonElement>) => {
    event.preventDefault();
    event.stopPropagation();

    setOpen(true);
  }, []);

  const { context, floatingStyles, middlewareData, refs } = useFloating({
    open,
    onOpenChange: setOpen,
    middleware: [
      offset(8),
      shift({ padding: 8 }),
      size({
        apply({ availableWidth, availableHeight, elements }) {
          elements.floating.style.maxWidth = `${Math.min(500, availableWidth)}px`;
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

  return (
    <>
      <HoverTooltip tipContent="View session summary">
        <ButtonBorder
          width="32px"
          padding="0"
          aria-label="View session summary"
          onClick={handleClick}
          ref={refs.setReference}
          {...getReferenceProps()}
        >
          <ChatCircleSparkle size="small" />
        </ButtonBorder>
      </HoverTooltip>

      {open && (
        <Modal open={true} BackdropProps={{ invisible: true }}>
          <StyledPopover
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

            <Box
              px={3}
              py={3}
              style={{ overflowY: 'auto' }}
              maxHeight="700px"
              width="500px"
              data-scrollbar="default"
            >
              <ErrorSuspenseWrapper
                errorComponent={SummaryErrorWrapper}
                loadingComponent={SummaryLoadingWrapper}
              >
                <SessionSummary sessionId={sessionId} />
              </ErrorSuspenseWrapper>
            </Box>
          </StyledPopover>
        </Modal>
      )}
    </>
  );
}

const SessionSummaryContainer = styled(Flex)`
  align-items: center;
  flex-direction: column;
  justify-content: center;
  width: 100%;
  gap: ${p => p.theme.space[2]}px;
`;

function SummaryErrorWrapper({ error, resetErrorBoundary }: FallbackProps) {
  return (
    <SessionSummaryContainer>
      <SessionSummaryError
        error={error}
        resetErrorBoundary={resetErrorBoundary}
      />
    </SessionSummaryContainer>
  );
}

function SummaryLoadingWrapper() {
  return (
    <SessionSummaryContainer>
      <SessionSummaryLoading />
    </SessionSummaryContainer>
  );
}

export function SessionSummaryError({
  error,
  resetErrorBoundary,
}: FallbackProps) {
  const errorMessage = getErrorMessage(error);

  if (errorMessage.includes('not found')) {
    return <Text>Could not find a summary for this session.</Text>;
  }

  return (
    <>
      <Text color="error.main">Error loading session summary</Text>

      <Text>{errorMessage}</Text>

      <Flex justifyContent="center">
        <ButtonSecondary onClick={resetErrorBoundary}>Retry</ButtonSecondary>
      </Flex>
    </>
  );
}

export function SessionSummaryLoading() {
  return (
    <>
      <Text color="text.slightlyMuted">Loading session summary...</Text>

      <Indicator delay="none" />
    </>
  );
}

interface SessionSummaryProps {
  sessionId: string;
}

const MarkdownContainer = styled.div`
  h1 {
    font-size: 20px;
  }

  h2 {
    font-size: 18px;
  }

  h3 {
    font-size: 16px;
  }
`;

const SummaryInfo = styled(Box)`
  background: ${p => p.theme.colors.levels.elevated};
  border-radius: ${p => p.theme.radii[3]}px;
  color: ${p => p.theme.colors.text.slightlyMuted};
  font-size: 13px;
  padding: ${p => p.theme.space[2]}px;
  margin-bottom: ${p => p.theme.space[3]}px;
  line-height: 1.4;
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
`;

export function SessionSummary({ sessionId }: SessionSummaryProps) {
  const { clusterId } = useStickyClusterId();

  const { data, isRefetching, refetch } = useSuspenseGetRecordingSummary({
    clusterId,
    sessionId,
  });

  const handleRefetch = useCallback(() => {
    void refetch();
  }, [refetch]);

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

        <ButtonSecondary onClick={handleRefetch} disabled={isRefetching} mt={1}>
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

    let content: ReactNode | null = null;
    if (isValid(inferenceStartedAt) && isValid(inferenceFinishedAt)) {
      const duration = intervalToDuration({
        start: inferenceStartedAt,
        end: inferenceFinishedAt,
      });

      content = <>Summarization took {formatDuration(duration)}.</>;
    }

    return (
      <MarkdownContainer>
        <H3>Session Summary</H3>

        <Markdown text={data.content} />

        <SummaryInfo>
          <strong>AI can make mistakes.</strong> {content}
        </SummaryInfo>
      </MarkdownContainer>
    );
  }

  return null;
}
