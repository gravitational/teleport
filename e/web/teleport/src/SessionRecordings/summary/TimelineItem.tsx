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
import { useCallback, useState } from 'react';
import styled, { useTheme } from 'styled-components';

import Box from 'design/Box';
import { Button, ButtonText } from 'design/Button';
import Flex from 'design/Flex';
import { ChevronRight, Cross } from 'design/Icon';
import Modal from 'design/Modal';
import { StyledPopover } from 'design/Popover';
import { Markdown } from 'shared/components/Markdown/Markdown';

import { type CommandAnalysis } from 'e-teleport/services/recordings/types';
import {
  getRiskColor,
  RiskLevel,
} from 'e-teleport/SessionRecordings/summary/RiskLevel';
import { RiskLevel as RiskLevelValue } from 'teleport/services/recordings/types';

interface TimelineItemProps {
  command: CommandAnalysis;
  selected: boolean;
  onOpenChange: (open: boolean) => void;
  onPlay?: () => void;
  nextRiskLevel?: RiskLevelValue;
}

export function TimelineItem({
  command,
  selected,
  onOpenChange,
  onPlay,
  nextRiskLevel,
}: TimelineItemProps) {
  const [arrowEl, setArrowEl] = useState<HTMLDivElement>(null);

  const { context, floatingStyles, middlewareData, refs } = useFloating({
    open: selected,
    onOpenChange,
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
    placement: 'right',
    whileElementsMounted: autoUpdate,
  });

  const dismiss = useDismiss(context);

  const handlePlay = useCallback(() => {
    if (onPlay) {
      onOpenChange(false);
      onPlay();
    }
  }, [onPlay, onOpenChange]);

  const { getReferenceProps, getFloatingProps } = useInteractions([dismiss]);

  const theme = useTheme();

  const riskLevelColor = getRiskColor(theme, command.riskLevel);
  const nextColor = getRiskColor(theme, nextRiskLevel);

  return (
    <>
      <Flex alignItems="flex-start" position="relative" zIndex={2}>
        <Flex
          fontFamily="mono"
          fontSize="11px"
          color="text.muted"
          fontWeight="100"
          width="30px"
          flexShrink={0}
          ml={3}
          pt="6px"
        >
          {formatOffset(command.startOffset ?? 0)}
        </Flex>

        <Box
          position="absolute"
          top="22px"
          bottom="-28px"
          left="61px"
          width="2px"
          backgroundImage={`linear-gradient(to bottom, ${riskLevelColor}, ${nextColor})`}
          zIndex={1}
        />

        <Flex
          width="32px"
          alignItems="center"
          justifyContent="center"
          position="relative"
          flexShrink={0}
          mt="12px"
          zIndex={2}
        >
          <Box
            backgroundColor={riskLevelColor}
            borderRadius="50%"
            color={riskLevelColor}
            width="10px"
            height="10px"
          />
        </Flex>

        <StyledBox
          backgroundColor={selected ? 'spotBackground.1' : null}
          px={2}
          py={1}
          borderRadius="8px"
          position="relative"
          border="1px solid"
          borderColor={selected ? 'brand' : 'spotBackground.2'}
          onClick={() => onOpenChange(!selected)}
          ref={refs.setReference}
          width="fit-content"
          {...getReferenceProps()}
        >
          <Markdown text={command.timelineTitle} />
        </StyledBox>
      </Flex>

      {selected && (
        <Modal open={true} BackdropProps={{ invisible: true }}>
          <StyledPopover
            shadow={true}
            ref={refs.setFloating}
            style={{ ...floatingStyles, overflow: 'visible' }}
            {...getFloatingProps()}
          >
            <Arrow
              ref={setArrowEl}
              style={{
                left: middlewareData.arrow?.x ?? '',
                top: middlewareData.arrow?.y ?? '',
              }}
            />

            <Box position="absolute" top={3} right={3}>
              <Button
                width="24px"
                size="small"
                padding="0"
                intent="neutral"
                aria-label="Close"
                onClick={() => onOpenChange(false)}
              >
                <Cross size="small" />
              </Button>
            </Box>

            <Box
              px={3}
              py={3}
              style={{ overflowY: 'auto' }}
              maxHeight="700px"
              width="500px"
              data-scrollbar="default"
            >
              <Flex alignItems="center" mb={3} mt={1} gap={2}>
                <RiskLevel riskLevel={command.riskLevel} inPopover={true} />
              </Flex>
              <Box
                fontFamily="mono"
                fontSize="13px"
                backgroundColor="spotBackground.0"
                px={2}
                py={1}
                borderRadius="6px"
                mt={2}
              >
                {command.command}
              </Box>

              <Markdown text={command.detailedDescription} />

              {onPlay && (
                <Flex justifyContent="flex-end" mt={3}>
                  <ButtonText px={2} onClick={handlePlay}>
                    Play in recording
                    <ChevronRight size="small" ml={1} />
                  </ButtonText>
                </Flex>
              )}
            </Box>
          </StyledPopover>
        </Modal>
      )}
    </>
  );
}

const StyledBox = styled(Box)`
  cursor: pointer;
  user-select: none;

  &:hover {
    background-color: ${p => p.theme.colors.spotBackground[1]};
  }

  p {
    margin: 0;
  }

  code {
    font-size: 0.9rem;
    background-color: ${p => p.theme.colors.spotBackground[1]};
    padding: 2px ${p => p.theme.space[1]}px;
    border-radius: 4px;
  }
`;

const Arrow = styled.div`
  position: absolute;
  width: 8px;
  left: -4px;
  height: 8px;
  top: 50%;
  margin-top: -4px;
  background: ${p => p.theme.colors.levels.elevated};
  transform: rotate(-45deg);
  z-index: -1;
  border-left: 1px solid ${p => p.theme.colors.spotBackground[1]};
  border-top: 1px solid ${p => p.theme.colors.spotBackground[1]};
`;

export function formatOffset(offset: number) {
  const totalSeconds = Math.floor(offset / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  if (hours > 0) {
    return `${hours}:${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
  }

  return `${minutes}:${seconds.toString().padStart(2, '0')}`;
}
