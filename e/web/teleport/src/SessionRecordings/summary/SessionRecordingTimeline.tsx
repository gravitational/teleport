import { formatDuration, type Duration } from 'date-fns';
import { useMemo, useState, type PropsWithChildren } from 'react';
import { useTheme } from 'styled-components';

import Box from 'design/Box';
import Flex from 'design/Flex';

import { type CommandAnalysis } from 'e-teleport/services/recordings/types';
import { getRiskColor } from 'e-teleport/SessionRecordings/summary/RiskLevel';
import {
  formatOffset,
  TimelineItem,
} from 'e-teleport/SessionRecordings/summary/TimelineItem';
import { RiskLevel } from 'teleport/services/recordings/types';

interface SessionRecordingTimelineProps {
  commands: CommandAnalysis[];
  onPlay?: (timestamp: number) => void;
  inferenceDuration: Duration | null;
  sessionDuration: number | null;
}

export function SessionRecordingTimeline({
  commands,
  onPlay,
  inferenceDuration,
  sessionDuration,
}: SessionRecordingTimelineProps) {
  const [selectedCommandIndex, setSelectedCommandIndex] = useState(-1);

  const items = useMemo(
    () =>
      commands.map((command, index) => (
        <TimelineItem
          key={index}
          command={command}
          selected={selectedCommandIndex === index}
          onOpenChange={(open: boolean) =>
            setSelectedCommandIndex(open ? index : -1)
          }
          onPlay={onPlay ? () => onPlay(command.startOffset ?? 0) : undefined}
          nextRiskLevel={commands[index + 1]?.riskLevel}
        />
      )),
    [commands, onPlay, selectedCommandIndex]
  );

  const lastTimestamp = sessionDuration
    ? sessionDuration
    : commands.length > 0
      ? commands[commands.length - 1].endOffset
      : 0;

  return (
    <Box>
      <Flex flexDirection="column" gap={3} position="relative" pr={3}>
        <Box
          position="absolute"
          top={0}
          left="30px"
          bottom={0}
          width="40px"
          zIndex={2}
        >
          <Box
            position="absolute"
            top="12px"
            bottom="12px"
            left="35px"
            width="2px"
            backgroundColor="spotBackground.2"
          />
        </Box>

        <StartMarker firstRiskLevel={commands[0]?.riskLevel}>
          Session started
        </StartMarker>

        <Flex flexDirection="column" gap={3} my={0}>
          {items}
        </Flex>

        <StartEndMarker offset={lastTimestamp}>Session ended</StartEndMarker>
      </Flex>

      {inferenceDuration && (
        <Box color="text.disabled" fontSize="small" mt={2} pl="10px">
          <strong>AI can make mistakes.</strong> Summarization took{' '}
          {formatDuration(inferenceDuration)}.
        </Box>
      )}
    </Box>
  );
}

interface StartMarkerProps {
  firstRiskLevel: RiskLevel | undefined;
}

function StartMarker({
  children,
  firstRiskLevel,
}: PropsWithChildren<StartMarkerProps>) {
  const theme = useTheme();

  const noneColor = getRiskColor(theme, RiskLevel.None);
  const riskLevelColor = getRiskColor(theme, firstRiskLevel);

  return (
    <StartEndMarker offset={0}>
      <Box
        position="absolute"
        top="14px"
        bottom="-28px"
        left="65px"
        width="2px"
        backgroundImage={`linear-gradient(to bottom, ${noneColor}, ${riskLevelColor})`}
        zIndex={1}
      />

      {children}
    </StartEndMarker>
  );
}

interface StartEndMarkerProps {
  offset: number | undefined;
}

function StartEndMarker({
  children,
  offset,
}: PropsWithChildren<StartEndMarkerProps>) {
  const noneColor = getRiskColor(useTheme(), RiskLevel.None);
  const timestamp = typeof offset !== 'undefined' ? formatOffset(offset) : '';

  return (
    <Flex alignItems="center" position="relative" zIndex={2} gap={1}>
      <Flex
        fontFamily="mono"
        fontSize="11px"
        color="text.muted"
        fontWeight="100"
        width="30px"
        flexShrink={0}
        ml={3}
        pt="2px"
      >
        {timestamp}
      </Flex>

      <Flex width="32px" alignItems="center" justifyContent="center">
        <Box
          width="8px"
          height="8px"
          borderRadius="100%"
          backgroundColor={noneColor}
        />
      </Flex>

      <Box color="text.muted" fontSize="13px">
        {children}
      </Box>
    </Flex>
  );
}
