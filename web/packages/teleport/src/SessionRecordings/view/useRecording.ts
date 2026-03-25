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

import { useCallback, useEffect, useMemo, useRef } from 'react';

import { useLocalStorage } from 'shared/hooks/useLocalStorage';

import { useFullscreen } from 'teleport/components/hooks/useFullscreen';
import { KeysEnum } from 'teleport/services/storageService';
import { DEFAULT_SIDEBAR_WIDTH } from 'teleport/SessionRecordings/view/SidebarResizeHandle';
import type { PlayerHandle } from 'teleport/SessionRecordings/view/SshPlayer';
import type { RecordingTimelineHandle } from 'teleport/SessionRecordings/view/Timeline/RecordingTimeline';

export function useRecording() {
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
  const [sidebarWidth, setSidebarWidth] = useLocalStorage(
    KeysEnum.SESSION_RECORDING_SIDEBAR_WIDTH,
    DEFAULT_SIDEBAR_WIDTH
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

  const goToTime = useCallback((time: number) => {
    if (!playerRef.current) {
      return;
    }

    currentTimeRef.current = time;
    playerRef.current.moveToTime(time);
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

  useEffect(() => {
    if (!timelineRef.current || timelineHidden) {
      return;
    }

    timelineRef.current.moveToTime(currentTimeRef.current);
  }, [timelineHidden]);

  return useMemo(
    () => ({
      containerRef,
      playerRef,
      timelineRef,
      fullscreen,
      timelineHidden,
      sidebarHidden,
      sidebarWidth,
      setSidebarWidth,
      goToTime,
      handleTimeChange,
      handleTimelineTimeChange,
      toggleSidebar,
      toggleTimeline,
      handleToggleFullscreen,
    }),
    [
      containerRef,
      playerRef,
      timelineRef,
      fullscreen,
      timelineHidden,
      sidebarHidden,
      sidebarWidth,
      setSidebarWidth,
      goToTime,
      handleTimeChange,
      handleTimelineTimeChange,
      toggleSidebar,
      toggleTimeline,
      handleToggleFullscreen,
    ]
  );
}
