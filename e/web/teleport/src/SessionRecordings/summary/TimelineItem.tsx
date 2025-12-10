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
import { useMemo, useState } from 'react';
import styled, { useTheme } from 'styled-components';

import Box from 'design/Box';
import { ButtonText } from 'design/Button';
import Flex from 'design/Flex';
import { ChevronRight } from 'design/Icon';
import Modal from 'design/Modal';
import { StyledPopover } from 'design/Popover';
import { Markdown } from 'shared/components/Markdown/Markdown';

import {
  RiskLevel,
  type CommandAnalysis,
} from 'e-teleport/services/recordings/types';

interface TimelineItemProps {
  command: CommandAnalysis;
  selected: boolean;
  onOpenChange: (open: boolean) => void;
  onPlay: () => void;
}

export function TimelineItem({
  command,
  selected,
  onOpenChange,
  onPlay,
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

  const { getReferenceProps, getFloatingProps } = useInteractions([dismiss]);

  const theme = useTheme();

  const riskLevelColor = useMemo(() => {
    switch (command.riskLevel) {
      case RiskLevel.Low:
        return theme.colors.success.main;
      case RiskLevel.Medium:
        return theme.colors.warning.main;
      case RiskLevel.High:
        return theme.colors.error.main;
      case RiskLevel.Critical:
        return theme.colors.error.main;
      default:
        return theme.colors.text.muted;
    }
  }, [command.riskLevel, theme]);

  return (
    <>
      <Flex alignItems="center" position="relative" zIndex={2}>
        <Flex
          width="60px"
          alignItems="center"
          justifyContent="center"
          position="relative"
        >
          <Box
            position="absolute"
            top="-30px"
            height="30px"
            left="50%"
            style={{ transform: 'translateX(-50%)' }}
            width="2px"
            backgroundImage={`linear-gradient(to top, ${riskLevelColor}, transparent)`}
          />

          <Box
            position="absolute"
            bottom="-30px"
            height="30px"
            left="50%"
            style={{ transform: 'translateX(-50%)' }}
            width="2px"
            backgroundImage={`linear-gradient(to bottom, ${riskLevelColor}, transparent)`}
          />

          <Box
            backgroundColor={riskLevelColor}
            borderRadius="50%"
            color={riskLevelColor}
            width="10px"
            height="10px"
          />
        </Flex>

        <StyledBox
          bg="spotBackground.0"
          px={2}
          py={1}
          borderRadius="8px"
          position="relative"
          border="1px solid"
          borderColor={selected ? 'brand' : 'spotBackground.2'}
          onClick={() => onOpenChange(!selected)}
          ref={refs.setReference}
          {...getReferenceProps()}
        >
          <Markdown text={command.timelineTitle} />
        </StyledBox>

        <Box
          fontFamily="mono"
          fontSize="11px"
          color="text.muted"
          fontWeight="100"
          ml={2}
        >
          {formatOffset(command.startOffset)}
        </Box>
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

            <Box
              px={3}
              py={3}
              style={{ overflowY: 'auto' }}
              maxHeight="700px"
              width="500px"
              data-scrollbar="default"
            >
              <Flex alignItems="center" mb={3} mt={1} gap={2}>
                <Flex
                  alignItems="center"
                  border="1px solid"
                  borderColor={riskLevelColor}
                  color={riskLevelColor}
                  fontWeight="500"
                  lineHeight={1}
                  height="24px"
                  px={2}
                  fontSize="small"
                  borderRadius="8px"
                >
                  {getRiskLevelLabel(command.riskLevel)}
                </Flex>
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

              <Flex justifyContent="flex-end" mt={3}>
                <ButtonText px={2} onClick={onPlay}>
                  Play in recording
                  <ChevronRight size="small" ml={1} />
                </ButtonText>
              </Flex>
            </Box>
          </StyledPopover>
        </Modal>
      )}
    </>
  );
}

function getRiskLevelLabel(riskLevel: RiskLevel) {
  switch (riskLevel) {
    case RiskLevel.Low:
      return 'Low';
    case RiskLevel.Medium:
      return 'Medium';
    case RiskLevel.High:
      return 'High';
    case RiskLevel.Critical:
      return 'Critical';
    default:
      return 'Unknown';
  }
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

function formatOffset(offset: number) {
  const totalSeconds = Math.floor(offset / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  if (hours > 0) {
    return `${hours}:${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
  }

  return `${minutes}:${seconds.toString().padStart(2, '0')}`;
}
