import { useCallback, useMemo, useState, type PropsWithChildren } from 'react';

import Box from 'design/Box';
import Flex from 'design/Flex';

import type { CommandAnalysis } from 'e-teleport/services/recordings/types';
import { TimelineItem } from 'e-teleport/SessionRecordings/summary/TimelineItem';

interface SessionRecordingTimelineProps {
  commands: CommandAnalysis[];
  onPlay: (timestamp: number) => void;
}

export function SessionRecordingTimeline({
  commands,
  onPlay,
}: SessionRecordingTimelineProps) {
  const [selectedCommandIndex, setSelectedCommandIndex] = useState(-1);

  const playCommand = useCallback(
    (commandIndex: number) => {
      const command = commands[commandIndex];

      if (command) {
        onPlay(command.startOffset);
      }
    },
    [commands, onPlay]
  );

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
          onPlay={() => playCommand(index)}
        />
      )),
    [commands, playCommand, selectedCommandIndex]
  );

  return (
    <Flex flexDirection="column" gap={3} position="relative">
      <Box
        position="absolute"
        top={0}
        left={0}
        bottom={0}
        width="60px"
        zIndex={1}
      >
        <Box
          position="absolute"
          top="10px"
          bottom="10px"
          left="50%"
          style={{ transform: 'translateX(-50%)' }}
          width="2px"
          backgroundColor="spotBackground.2"
        />
      </Box>

      <StartEndMarker>Session started</StartEndMarker>

      {items}

      <StartEndMarker>Session ended</StartEndMarker>
    </Flex>
  );
}

function StartEndMarker({ children }: PropsWithChildren) {
  return (
    <Flex alignItems="center" position="relative" zIndex={2}>
      <Flex width="60px" alignItems="center" justifyContent="center">
        <Box
          width="6px"
          height="6px"
          borderRadius="100%"
          backgroundColor="text.muted"
        />
      </Flex>

      <Box color="text.muted" fontSize="13px">
        {children}
      </Box>
    </Flex>
  );
}
