/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import {
  useCallback,
  useImperativeHandle,
  useRef,
  type MouseEvent,
  type RefObject,
} from 'react';
import styled from 'styled-components';

import Box from 'design/Box';
import Flex from 'design/Flex';
import {
  CirclePause,
  CirclePlay,
  CornersIn,
  CornersOut,
  FilmStrip,
  Layout,
  Refresh,
} from 'design/Icon';
import { Indicator } from 'design/Indicator';
import { HoverTooltip } from 'design/Tooltip';

import { PlayerState } from 'teleport/SessionRecordings/view/stream/SessionStream';

import { PlayerSpeed } from './PlayerSpeed';

export interface PlayerControlsHandle {
  setTime: (time: number) => void;
}

interface PlayerControlsProps {
  duration: number;
  onPlay: () => void;
  onPause: () => void;
  onSeek: (time: number) => void;
  speed: number;
  onSpeedChange: (speed: number) => void;
  state: PlayerState;
  ref: RefObject<PlayerControlsHandle>;
  onToggleFullscreen?: () => void;
  fullscreen?: boolean;
  onToggleTimeline?: () => void;
  onToggleSidebar?: () => void;
}

export const CONTROLS_HEIGHT = 42;

export function PlayerControls({
  duration,
  onPlay,
  onPause,
  onSeek,
  speed,
  onSpeedChange,
  fullscreen,
  onToggleFullscreen,
  onToggleTimeline,
  onToggleSidebar,
  state,
  ref,
}: PlayerControlsProps) {
  const buttonDisabled = state === PlayerState.Loading;

  const containerRef = useRef<HTMLDivElement>(null);
  const durationRef = useRef<HTMLDivElement>(null);
  const progressBarRef = useRef<HTMLDivElement>(null);
  const progressBarContainerRef = useRef<HTMLDivElement>(null);
  const seekingRef = useRef<HTMLDivElement>(null);

  const setTime = useCallback(
    (time: number) => {
      if (
        !durationRef.current ||
        !progressBarRef.current ||
        !progressBarContainerRef.current
      ) {
        return;
      }

      const containerWidth = progressBarContainerRef.current.clientWidth;
      const progressBarWidth = (time / duration) * containerWidth;

      progressBarRef.current.style.width = `${Math.min(progressBarWidth, containerWidth)}px`;

      durationRef.current.textContent = formatDuration(time);
    },
    [duration]
  );

  useImperativeHandle(ref, () => ({
    setTime,
  }));

  const handleProgressBarClick = useCallback(
    (e: MouseEvent<HTMLDivElement>) => {
      if (!progressBarContainerRef.current || !onSeek) {
        return;
      }

      const containerRect =
        progressBarContainerRef.current.getBoundingClientRect();
      const relativeX = e.clientX - containerRect.left;
      const percentage = Math.min(
        Math.max(relativeX / containerRect.width, 0),
        1
      );
      const time = percentage * duration;

      onSeek(Math.floor(time));
    },
    [duration, onSeek]
  );

  const handleProgressBarMouseMove = useCallback(
    (e: MouseEvent<HTMLDivElement>) => {
      if (!progressBarContainerRef.current || !seekingRef.current) {
        return;
      }

      const progressBarContainerRect =
        progressBarContainerRef.current.getBoundingClientRect();
      const relativeX = e.clientX - progressBarContainerRect.left;
      const percentage = Math.min(
        Math.max(relativeX / progressBarContainerRect.width, 0),
        1
      );
      const time = percentage * duration;

      seekingRef.current.style.width = `${relativeX}px`;
      seekingRef.current.style.opacity = '1';
      seekingRef.current.textContent = formatDuration(time);
    },
    [duration]
  );

  const handleProgressBarMouseLeave = useCallback(() => {
    if (!seekingRef.current) {
      return;
    }

    seekingRef.current.style.opacity = '0';
  }, []);

  const handlePlayerButtonClick = useCallback(() => {
    if (state === PlayerState.Playing) {
      onPause();
    } else if (state === PlayerState.Paused) {
      onPlay();
    } else if (state === PlayerState.Stopped) {
      onSeek(0);
      onPlay();
    }
  }, [state, onPause, onPlay, onSeek]);

  return (
    <Flex
      alignItems="center"
      bg="levels.surface"
      borderTop="1px solid"
      borderColor="spotBackground.0"
      height={CONTROLS_HEIGHT}
      px={2}
      flexShrink={0}
      ref={containerRef}
    >
      {onToggleSidebar && (
        <HoverTooltip
          tipContent="Toggle Sidebar"
          portalRoot={containerRef.current}
        >
          <PlayerButton onClick={onToggleSidebar}>
            <Layout size="small" />
          </PlayerButton>
        </HoverTooltip>
      )}

      <PlayButton
        disabled={buttonDisabled}
        onClick={handlePlayerButtonClick}
        state={state}
        containerRef={containerRef}
      />

      <Box
        fontFamily="mono"
        fontSize="small"
        pl={2}
        pr={4}
        ref={durationRef}
        width="70px"
      >
        0:00
      </Box>

      <ProgressBarArea
        onClick={handleProgressBarClick}
        onMouseLeave={handleProgressBarMouseLeave}
        onMouseMove={handleProgressBarMouseMove}
      >
        <ProgressBarContainer ref={progressBarContainerRef}>
          <SeekingBar ref={seekingRef} />
          <ProgressBar ref={progressBarRef} />
        </ProgressBarContainer>
      </ProgressBarArea>

      <PlayerSpeed
        speed={speed}
        onSpeedChange={onSpeedChange}
        portalRoot={containerRef.current}
      />

      {onToggleTimeline && (
        <HoverTooltip
          tipContent="Toggle Timeline"
          portalRoot={containerRef.current}
        >
          <PlayerButton onClick={onToggleTimeline}>
            <FilmStrip size="small" />
          </PlayerButton>
        </HoverTooltip>
      )}

      {onToggleFullscreen && (
        <HoverTooltip
          tipContent={fullscreen ? 'Exit Full Screen' : 'Full Screen'}
          portalRoot={containerRef.current}
        >
          <PlayerButton onClick={onToggleFullscreen}>
            {fullscreen ? (
              <CornersIn size="small" />
            ) : (
              <CornersOut size="small" />
            )}
          </PlayerButton>
        </HoverTooltip>
      )}
    </Flex>
  );
}

interface PlayButtonProps {
  disabled: boolean;
  onClick: () => void;
  state: PlayerState;
  containerRef: RefObject<HTMLDivElement>;
}

function PlayButton({
  disabled,
  onClick,
  state,
  containerRef,
}: PlayButtonProps) {
  const { icon: Icon, text } = getIconAndText(state);

  return (
    <HoverTooltip tipContent={text} portalRoot={containerRef.current}>
      <PlayerButton disabled={disabled} onClick={onClick}>
        <Icon size="small" color="text.main" />
      </PlayerButton>
    </HoverTooltip>
  );
}

const AdjustedIndicator = styled(Indicator)`
  position: relative;
  top: 1px; // the indicator is just so slightly off center vertically with the rest of the icons
`;

function getIconAndText(state: PlayerState) {
  switch (state) {
    case PlayerState.Playing:
      return { icon: CirclePause, text: 'Pause' };
    case PlayerState.Paused:
      return { icon: CirclePlay, text: 'Play' };
    case PlayerState.Stopped:
      return { icon: Refresh, text: 'Replay' };
    case PlayerState.Loading:
      return { icon: AdjustedIndicator };
  }
}

const SeekingBar = styled.div`
  background: ${p =>
    p.theme.colors.sessionRecording.player.progressBar.seeking};
  height: 6px;
  left: 0;
  position: absolute;
  top: 0;
  width: 0;
  z-index: 3;
`;

const ProgressBar = styled.div`
  background: ${p =>
    p.theme.colors.sessionRecording.player.progressBar.progress};
  height: 6px;
  left: 0;
  position: absolute;
  top: 0;
  width: 0;
  z-index: 3;
`;

const ProgressBarArea = styled.div`
  height: 100%;
  flex: 1;
  cursor: pointer;
  display: flex;
  align-items: center;
  margin-right: ${p => p.theme.space[3]}px;
`;

const ProgressBarContainer = styled.div`
  background: ${p =>
    p.theme.colors.sessionRecording.player.progressBar.background};
  border-radius: 3px;
  height: 6px;
  overflow: hidden;
  position: relative;
  width: 100%;
`;

const PlayerButton = styled.button<{ disabled?: boolean }>`
  background: transparent;
  border: none;
  color: ${p => p.theme.colors.text.main};
  width: ${p => p.theme.space[5]}px;
  height: ${p => p.theme.space[5]}px;
  border-radius: ${p => p.theme.radii[3]}px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  pointer-events: ${p => (p.disabled ? 'none' : 'auto')};

  &:hover {
    background-color: ${p =>
      p.disabled ? 'none' : p.theme.colors.spotBackground[1]};
  }
`;

function formatDuration(ms: number): string {
  const hours = Math.floor(ms / 3600000);
  const minutes = Math.floor(ms / 60000);
  const secs = Math.floor((ms % 60000) / 1000);

  if (hours > 0) {
    return `${hours}:${minutes.toString().padStart(2, '0')}:${secs
      .toString()
      .padStart(2, '0')}`;
  }

  return `${minutes}:${secs.toString().padStart(2, '0')}`;
}
