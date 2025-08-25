import { format } from 'date-fns';
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';

import Flex from 'design/Flex';
import {
  ArrowLineLeft,
  ChevronLeft,
  DotsThreeMoreVertical,
  Server,
  Terminal,
  User,
} from 'design/Icon';
import Text, { H3 } from 'design/Text';
import { HoverTooltip } from 'design/Tooltip';
import { useLocalStorage } from 'shared/hooks/useLocalStorage';

import cfg from 'teleport/config';
import { useSuspenseGetRecordingMetadata } from 'teleport/services/recordings/hooks';
import { KeysEnum } from 'teleport/services/storageService';
import { formatSessionRecordingDuration } from 'teleport/SessionRecordings/list/RecordingItem';
import { RecordingPlayer } from 'teleport/SessionRecordings/view/RecordingPlayer';
import type { PlayerHandle } from 'teleport/SessionRecordings/view/SshPlayer';
import { KeyboardShortcuts } from 'teleport/SessionRecordings/view/Timeline/KeyboardShortcuts';

import {
  RecordingTimeline,
  type RecordingTimelineHandle,
} from './Timeline/RecordingTimeline';

export type SummarySlot = (sessionId: string) => ReactNode;

interface RecordingWithMetadataProps {
  clusterId: string;
  sessionId: string;
  summarySlot?: SummarySlot;
}

const Grid = styled.div<{ sidebarHidden: boolean }>`
  display: grid;
  grid-template-areas: ${p =>
    p.sidebarHidden
      ? `'recording recording' 'timeline timeline'`
      : `'sidebar recording' 'timeline timeline'`};
  grid-template-columns: 1fr 4fr;
  grid-template-rows: 1fr auto;
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
`;

const InfoGrid = styled.div`
  display: grid;
  column-gap: ${p => p.theme.space[3]}px;
  row-gap: ${p => p.theme.space[2]}px;
  grid-template-columns: 80px 1fr;
  padding: 0 ${p => p.theme.space[3]}px;
`;

const Player = styled.div`
  grid-area: recording;
  display: flex;
  justify-content: center;
  align-items: center;
  position: relative;
`;

const Sidebar = styled.div`
  grid-area: sidebar;
  border-right: 1px solid ${p => p.theme.colors.spotBackground[1]};
`;

const Summary = styled.div`
  border-top: 1px solid ${p => p.theme.colors.spotBackground[1]};
`;

const InfoGridLabel = styled.div`
  font-weight: bold;
  color: ${p => p.theme.colors.text.slightlyMuted};
`;

const BackLink = styled(Link)`
  color: ${p => p.theme.colors.text.slightlyMuted};
  text-decoration: none;
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: ${p => p.theme.space[2]}px;
`;

const HideSidebarButton = styled.button`
  background: none;
  border: none;
  color: ${p => p.theme.colors.text.slightlyMuted};
  cursor: pointer;
  padding: ${p => p.theme.space[2]}px;
  border-radius: ${p => p.theme.radii[2]}px;
  display: flex;
  align-items: center;

  &:hover {
    background: ${p => p.theme.colors.spotBackground[1]};
  }
`;

const ShowSidebarButton = styled(HideSidebarButton)`
  background: ${p => p.theme.colors.levels.surface};
  border: 1px solid ${p => p.theme.colors.spotBackground[2]};
  border-left: none;
  opacity: 0.6;
  position: absolute;
  top: 50%;
  left: 0;
  transform: translateY(-50%);
  z-index: 10;
  border-radius: 0 ${p => p.theme.radii[3]}px ${p => p.theme.radii[3]}px 0;
  padding: ${p => p.theme.space[2]}px 0;

  &:hover {
    opacity: 1;
  }
`;

const TimelineContainer = styled.div`
  grid-area: timeline;
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

export function RecordingWithMetadata({
  clusterId,
  sessionId,
  summarySlot,
}: RecordingWithMetadataProps) {
  const { data } = useSuspenseGetRecordingMetadata({
    clusterId,
    sessionId,
  });

  const playerRef = useRef<PlayerHandle>(null);
  const timelineRef = useRef<RecordingTimelineHandle>(null);

  const [timelineHidden, setTimelineHidden] = useLocalStorage(
    KeysEnum.SESSION_RECORDING_TIMELINE_HIDDEN,
    false
  );
  const [sidebarHidden, setSidebarHidden] = useLocalStorage(
    KeysEnum.SESSION_RECORDING_SIDEBAR_HIDDEN,
    false
  );
  const [showAbsoluteTime, setShowAbsoluteTime] = useLocalStorage(
    KeysEnum.SESSION_RECORDING_TIMELINE_SHOW_ABSOLUTE_TIME,
    false
  );

  const [keyboardShortcutsOpen, setKeyboardShortcutsOpen] = useState(false);

  const toggleSidebar = useCallback(() => {
    setSidebarHidden(!sidebarHidden);
  }, [setSidebarHidden, sidebarHidden]);

  const summary = useMemo(
    () => (summarySlot ? summarySlot(sessionId) : null),
    [summarySlot, sessionId]
  );

  const startTime = new Date(data.metadata.startTime * 1000);
  const endTime = new Date(data.metadata.endTime * 1000);

  const handleTimeChange = useCallback((time: number) => {
    if (!timelineRef.current) {
      return;
    }

    timelineRef.current.moveToTime(time);
  }, []);

  const handleTimelineTimeChange = useCallback((time: number) => {
    if (!playerRef.current || !timelineRef.current) {
      return;
    }

    playerRef.current.moveToTime(time);
    timelineRef.current.moveToTime(time);
  }, []);

  const handleHideTimeline = useCallback(() => {
    setTimelineHidden(true);
  }, [setTimelineHidden]);

  const showTimeline = useMemo(() => {
    if (!timelineHidden) {
      return;
    }

    return () => setTimelineHidden(false);
  }, [timelineHidden, setTimelineHidden]);

  const handleCloseKeyboardShortcuts = useCallback(() => {
    setKeyboardShortcutsOpen(false);
  }, []);

  const handleOpenKeyboardShortcuts = useCallback(() => {
    setKeyboardShortcutsOpen(true);
  }, []);

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      switch (event.key) {
        case 't':
          setTimelineHidden(!timelineHidden);
          break;
        case 'a':
          setShowAbsoluteTime(!showAbsoluteTime);
          break;
        case 's':
          setSidebarHidden(!sidebarHidden);
          break;
        case '?':
          setKeyboardShortcutsOpen(prev => !prev);
          break;
      }
    }

    window.addEventListener('keydown', handleKeyDown);

    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [
    setShowAbsoluteTime,
    setSidebarHidden,
    setTimelineHidden,
    showAbsoluteTime,
    sidebarHidden,
    timelineHidden,
  ]);

  return (
    <>
      <Grid sidebarHidden={sidebarHidden}>
        <Player>
          <RecordingPlayer
            clusterId={clusterId}
            sessionId={sessionId}
            durationMs={data.metadata.duration}
            onTimeChange={handleTimeChange}
            recordingType="ssh"
            ref={playerRef}
            showTimeline={showTimeline}
          />
        </Player>

        {sidebarHidden ? (
          <HoverTooltip tipContent="Show Sidebar" placement="right">
            <ShowSidebarButton onClick={toggleSidebar}>
              <DotsThreeMoreVertical size="large" />
            </ShowSidebarButton>
          </HoverTooltip>
        ) : (
          <Sidebar>
            <Flex flexDirection="column" gap={4} pt={3}>
              <Flex pl={3} pr={2} justifyContent="space-between">
                <BackLink to={cfg.getRecordingsRoute(clusterId)}>
                  <ChevronLeft size="small" />
                  Back to Session Recordings
                </BackLink>

                <HoverTooltip tipContent="Hide Sidebar">
                  <HideSidebarButton onClick={toggleSidebar}>
                    <ArrowLineLeft size="small" />
                  </HideSidebarButton>
                </HoverTooltip>
              </Flex>

              <Flex alignItems="center" gap={3} px={3}>
                <Terminal />

                <H3>SSH Session</H3>
              </Flex>

              <InfoGrid>
                <InfoGridLabel>User</InfoGridLabel>

                <div>
                  <ItemSpan>
                    <User size="small" color="sessionRecording.user" />

                    <Text>{data.metadata.user}</Text>
                  </ItemSpan>
                </div>

                <InfoGridLabel>Resource</InfoGridLabel>

                <div>
                  <ItemSpan>
                    <Server size="small" color="sessionRecording.resource" />

                    <Text>{data.metadata.resourceName}</Text>
                  </ItemSpan>
                </div>

                <InfoGridLabel>Duration</InfoGridLabel>

                <div>
                  {formatSessionRecordingDuration(data.metadata.duration)}
                </div>

                <InfoGridLabel>Cluster</InfoGridLabel>

                <div>{data.metadata.clusterName}</div>

                <InfoGridLabel>Start Time</InfoGridLabel>

                <div>{format(startTime, 'MMM dd, yyyy HH:mm')}</div>

                <InfoGridLabel>End Time</InfoGridLabel>

                <div>{format(endTime, 'MMM dd, yyyy HH:mm')}</div>
              </InfoGrid>

              {summary && <Summary>{summary}</Summary>}
            </Flex>
          </Sidebar>
        )}

        {data.frames.length > 0 && !timelineHidden && (
          <TimelineContainer>
            <RecordingTimeline
              frames={data.frames}
              metadata={data.metadata}
              onHide={handleHideTimeline}
              onOpenKeyboardShortcuts={handleOpenKeyboardShortcuts}
              onTimeChange={handleTimelineTimeChange}
              ref={timelineRef}
              showAbsoluteTime={showAbsoluteTime}
            />
          </TimelineContainer>
        )}
      </Grid>

      <KeyboardShortcuts
        open={keyboardShortcutsOpen}
        onClose={handleCloseKeyboardShortcuts}
      />
    </>
  );
}
