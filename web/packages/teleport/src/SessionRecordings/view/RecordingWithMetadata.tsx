/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useCallback, useEffect, useMemo, useRef, type ReactNode } from 'react';
import styled from 'styled-components';

import Flex from 'design/Flex';
import { useLocalStorage } from 'shared/hooks/useLocalStorage';

import { useFullscreen } from 'teleport/components/hooks/useFullscreen';
import {
  type RecordingType,
  type SessionRecordingMetadata,
} from 'teleport/services/recordings';
import { useSuspenseGetRecordingMetadata } from 'teleport/services/recordings/hooks';
import { KeysEnum } from 'teleport/services/storageService';
import { RecordingPlayer } from 'teleport/SessionRecordings/view/RecordingPlayer';
import type { PlayerHandle } from 'teleport/SessionRecordings/view/SshPlayer';
import {
  RecordingTimeline,
  type RecordingTimelineHandle,
} from 'teleport/SessionRecordings/view/Timeline/RecordingTimeline';

export type SidebarSlot = (
  sessionId: string,
  recordingType: RecordingType,
  metadata: SessionRecordingMetadata | null,
  onPlay: (timestamp: number) => void
) => ReactNode;

interface RecordingWithMetadataProps {
  clusterId: string;
  sessionId: string;
  sidebarSlot: SidebarSlot;
}

export function RecordingWithMetadata({
  clusterId,
  sessionId,
  sidebarSlot,
}: RecordingWithMetadataProps) {
  const { data } = useSuspenseGetRecordingMetadata({
    clusterId,
    sessionId,
  });

  const currentTimeRef = useRef(0);
  const containerRef = useRef<HTMLDivElement>(null);
  const playerRef = useRef<PlayerHandle>(null);
  const timelineRef = useRef<RecordingTimelineHandle>(null);

  const fullscreen = useFullscreen(containerRef);

  const [timelineHidden, setTimelineHidden] = useLocalStorage(
    KeysEnum.SESSION_RECORDING_TIMELINE_HIDDEN,
    false
  );
  const [sidebarHidden, setSidebarHidden] = useLocalStorage(
    KeysEnum.SESSION_RECORDING_SIDEBAR_HIDDEN,
    false
  );

  // handle a time change from the player (update the timeline)
  const handleTimeChange = useCallback((time: number) => {
    if (!timelineRef.current) {
      return;
    }

    currentTimeRef.current = time;
    timelineRef.current.moveToTime(time);
  }, []);

  // handle a time change (user click) from the timeline (update the player and timeline)
  const handleTimelineTimeChange = useCallback((time: number) => {
    if (!playerRef.current || !timelineRef.current) {
      return;
    }

    currentTimeRef.current = time;
    playerRef.current.moveToTime(time);
    timelineRef.current.moveToTime(time);
  }, []);

  const toggleSidebar = useCallback(() => {
    // setSidebarHidden(prev => !prev) does not work with useLocalStorage, it stops working after the first toggle
    setSidebarHidden(!sidebarHidden);
  }, [sidebarHidden, setSidebarHidden]);

  const toggleTimeline = useCallback(() => {
    setTimelineHidden(!timelineHidden);
  }, [timelineHidden, setTimelineHidden]);

  const handleToggleFullscreen = useCallback(() => {
    if (fullscreen.active) {
      void fullscreen.exit();
    } else {
      void fullscreen.enter();
    }
  }, [fullscreen]);

  const sidebar = useMemo(
    () =>
      sidebarSlot(
        sessionId,
        data.metadata.type,
        data.metadata,
        handleTimelineTimeChange
      ),
    [sidebarSlot, sessionId, data.metadata, handleTimelineTimeChange]
  );

  useEffect(() => {
    if (!timelineRef.current || timelineHidden) {
      return;
    }

    timelineRef.current.moveToTime(currentTimeRef.current);
  }, [timelineHidden]);

  return (
    <Grid sidebarHidden={sidebarHidden} ref={containerRef}>
      <Player>
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
      </Player>

      {!sidebarHidden && (
        <Sidebar>
          <Flex
            flexDirection="column"
            gap={4}
            pt={3}
            minHeight={0}
            height="100%"
          >
            {sidebar}
          </Flex>
        </Sidebar>
      )}

      {data.frames.length > 0 && !timelineHidden && (
        <TimelineContainer>
          <RecordingTimeline
            frames={data.frames}
            metadata={data.metadata}
            onTimeChange={handleTimelineTimeChange}
            ref={timelineRef}
            showAbsoluteTime={false} // TODO(ryan): add with the keyboard shortcuts PR
          />
        </TimelineContainer>
      )}
    </Grid>
  );
}

const Grid = styled.div<{ sidebarHidden: boolean }>`
  background: ${p => p.theme.colors.levels.sunken};
  display: grid;
  grid-template-areas: ${p =>
    p.sidebarHidden
      ? `'recording recording' 'timeline timeline'`
      : `'sidebar recording' 'timeline timeline'`};
  grid-template-columns: 1fr 4fr;
  grid-template-rows: 1fr auto;
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
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
  overflow: hidden;
  display: flex;
  flex-direction: column;
`;

const TimelineContainer = styled.div`
  grid-area: timeline;
`;
