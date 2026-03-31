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
  ComponentType,
  useCallback,
  useState,
  type RefAttributes,
} from 'react';
import styled, { useTheme } from 'styled-components';

import { Alert } from 'design/Alert';
import Box from 'design/Box';
import { Button, ButtonText } from 'design/Button';
import Flex from 'design/Flex';
import { ChevronRight, Cross, WarningCircle } from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';
import Modal from 'design/Modal';
import { StyledPopover } from 'design/Popover';
import { Markdown } from 'shared/components/Markdown/Markdown';

import {
  formatCommandCategory,
  formatThreatCategory,
  getCommandCategoryIcon,
  getThreatCategoryIcon,
  ThreatCategory,
  type CommandAnalysis,
} from 'e-teleport/services/recordings/types';
import {
  getRiskColor,
  RiskLevel,
} from 'e-teleport/SessionRecordings/summary/RiskLevel';
import { RiskLevel as RiskLevelValue } from 'teleport/services/recordings/types';

import { RiskScore } from './RiskScore';

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

  const hasError = !!command.inferenceErrorMessage;

  const CommandCategoryIcon = getCommandCategoryIcon(command.category);

  return (
    <>
      <Flex alignItems="flex-start" position="relative" zIndex={2} gap={1}>
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
          top="30px"
          bottom="-26px"
          left="65px"
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
          mt="6px"
          zIndex={2}
        >
          <Flex
            backgroundColor="levels.sunken"
            alignItems="center"
            justifyContent="center"
            border="1px solid"
            borderColor={riskLevelColor}
            borderRadius="8px"
            color={riskLevelColor}
            width="24px"
            height="24px"
          >
            {hasError ? (
              <WarningCircle
                color={getRiskColor(theme, RiskLevelValue.High)}
                size="small"
              />
            ) : (
              <CommandCategoryIcon size="small" />
            )}
          </Flex>
        </Flex>

        <StyledBox
          backgroundColor={selected ? 'spotBackground.1' : null}
          px={2}
          py={1}
          borderRadius="8px"
          position="relative"
          border="1px solid"
          borderColor={
            hasError
              ? getRiskColor(theme, RiskLevelValue.High)
              : selected
                ? 'brand'
                : 'spotBackground.2'
          }
          onClick={() => onOpenChange(!selected)}
          ref={refs.setReference}
          minWidth={0}
          overflow="hidden"
          {...getReferenceProps()}
        >
          {command.timelineTitle ? (
            <Markdown text={command.timelineTitle} />
          ) : (
            <RawCommand>{command.command}</RawCommand>
          )}
        </StyledBox>
      </Flex>

      {selected && (
        <Modal open={true} BackdropProps={{ invisible: true }}>
          <StyledPopover
            shadow={true}
            ref={refs.setFloating}
            style={{
              ...floatingStyles,
              overflow: 'visible',
              borderRadius: '12px',
              border: `1px solid ${theme.colors.spotBackground[1]}`,
            }}
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
              pb={1}
              style={{ overflowY: 'auto' }}
              maxHeight="700px"
              width="500px"
              data-scrollbar="default"
            >
              <Flex
                alignItems="center"
                py={2}
                pl={2}
                pr={2}
                gap={2}
                flexWrap="wrap"
              >
                {!hasError && (
                  <>
                    <StyledBadge
                      Icon={CommandCategoryIcon}
                      label={formatCommandCategory(command.category)}
                    />
                    {command.threatCategory !== ThreatCategory.None && (
                      <StyledBadge
                        Icon={getThreatCategoryIcon(command.threatCategory)}
                        label={
                          'Threat: ' +
                          formatThreatCategory(command.threatCategory)
                        }
                        bordered
                      />
                    )}
                  </>
                )}

                <div style={{ flex: 1 }} />

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
              </Flex>

              <Divider mb={2} />

              {!hasError && (
                <Flex
                  alignItems="center"
                  mb={2}
                  gap={2}
                  px={3}
                  justifyContent="space-between"
                  flexWrap="wrap"
                >
                  <RiskLevel riskLevel={command.riskLevel} inPopover={true} />
                  <RiskScore
                    score={command.riskScore ?? 0}
                    riskLevel={command.riskLevel}
                  />
                </Flex>
              )}

              <Box
                fontFamily="mono"
                fontSize="13px"
                backgroundColor="spotBackground.0"
                px={2}
                py={1}
                borderRadius="8px"
                mt={2}
                mx={3}
                style={{ wordBreak: 'break-all' }}
              >
                {command.command}
              </Box>

              {hasError && (
                <Alert kind="danger" mx={3} mt={3}>
                  <Box>There was an error analyzing this command:</Box>
                  {command.inferenceErrorMessage}
                </Alert>
              )}

              {!hasError && (
                <>
                  <Divider my={3} />
                  <MarkdownContainer px={3}>
                    <Markdown text={command.detailedDescription} />
                  </MarkdownContainer>
                </>
              )}

              {onPlay && (
                <Flex justifyContent="flex-end" mt={3} pr={1}>
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

interface StyledBadgeProps {
  Icon: ComponentType<IconProps>;
  label: string;
  bordered?: boolean;
}

export function StyledBadge({
  Icon,
  label,
  bordered,
  ref,
}: StyledBadgeProps & RefAttributes<HTMLDivElement>) {
  return (
    <Flex
      inline
      alignItems="center"
      gap={1}
      backgroundColor="spotBackground.1"
      color="text.slightlyMuted"
      fontWeight="500"
      lineHeight={1}
      height="24px"
      px={2}
      fontSize="small"
      borderRadius="8px"
      border={bordered ? '1px solid' : 'none'}
      borderColor={bordered ? 'text.muted' : undefined}
      ref={ref}
    >
      <Icon size="small" />
      {label}
    </Flex>
  );
}

const MarkdownContainer = styled(Box)`
  p {
    margin: 0;
  }

  p + p {
    margin-top: ${p => p.theme.space[2]}px;
  }

  code {
    font-size: 0.9rem;
    background-color: ${p => p.theme.colors.spotBackground[1]};
    padding: 2px ${p => p.theme.space[1]}px;
    border-radius: 4px;
  }
`;

const Divider = styled(Box)`
  height: 1px;
  flex: 0;
  width: 100%;
  background-color: ${p => p.theme.colors.spotBackground[2]};
`;

const RawCommand = styled(Box)`
  font-family: ${p => p.theme.fonts.mono};
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 13px;
  min-width: 0;
`;

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
