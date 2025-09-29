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
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
  type RefObject,
} from 'react';
import styled, { keyframes } from 'styled-components';
import { useEventListener } from 'usehooks-ts';
import Box from 'web/packages/design/src/Box';
import Flex from 'web/packages/design/src/Flex';
import { Pause, Play } from 'web/packages/design/src/Icon';

import type { SessionRecordingEvent } from 'teleport/services/recordings';
import {
  CurrentEventInfo,
  type CurrentEventInfoHandle,
} from 'teleport/SessionRecordings/view/CurrentEventInfo';
import type { Player } from 'teleport/SessionRecordings/view/player/Player';
import {
  PlayerControls,
  type PlayerControlsHandle,
} from 'teleport/SessionRecordings/view/player/PlayerControls';
import type { PlayerHandle } from 'teleport/SessionRecordings/view/SshPlayer';
import {
  PlayerState,
  SessionStream,
} from 'teleport/SessionRecordings/view/stream/SessionStream';
import type { BaseEvent } from 'teleport/SessionRecordings/view/stream/types';

export interface RecordingPlayerProps<
  TEvent extends BaseEvent<TEventType>,
  TEventType extends number = number,
  TEndEventType extends TEventType = TEventType,
> {
  duration: number;
  onTimeChange: (time: number) => void;
  onToggleSidebar?: () => void;
  onToggleTimeline?: () => void;
  onToggleFullscreen?: () => void;
  fullscreen?: boolean;
  player: Player<TEvent>;
  endEventType: TEndEventType;
  decodeEvent: (buffer: ArrayBuffer) => TEvent;
  ref: RefObject<PlayerHandle>;
  events?: SessionRecordingEvent[];
  ws: WebSocket;
}

export function RecordingPlayer<
  TEvent extends BaseEvent<TEventType>,
  TEventType extends number = number,
>({
  duration,
  onTimeChange,
  player,
  endEventType,
  fullscreen,
  decodeEvent,
  onToggleFullscreen,
  onToggleSidebar,
  onToggleTimeline,
  events,
  ref,
  ws,
}: RecordingPlayerProps<TEvent>) {
  const [playerState, setPlayerState] = useState(PlayerState.Loading);

  const [showPlayButton, setShowPlayButton] = useState(true);

  const eventInfoRef = useRef<CurrentEventInfoHandle>(null);
  const controlsRef = useRef<PlayerControlsHandle>(null);
  const playerRef = useRef<HTMLDivElement>(null);

  const stream = useMemo(
    () => new SessionStream(ws, player, decodeEvent, endEventType, duration),
    [ws, player, decodeEvent, endEventType, duration]
  );

  useEffect(() => {
    stream.on('state', next => {
      setPlayerState(next);
    });

    stream.on('time', time => {
      if (!controlsRef.current || !eventInfoRef.current) {
        return;
      }

      controlsRef.current.setTime(time);
      onTimeChange(time);
      eventInfoRef.current.setTime(time);
    });

    stream.loadInitial();

    return () => {
      stream.destroy();
    };
  }, [stream, onTimeChange]);

  useEffect(() => {
    if (!playerRef.current) {
      return;
    }

    player.init(playerRef.current);

    const observer = new ResizeObserver(() => {
      player.fit();
    });

    observer.observe(playerRef.current);

    return () => {
      observer.disconnect();
    };
  }, [player]);

  const handlePlay = useCallback(() => {
    setShowPlayButton(false);

    stream.play();
  }, [stream]);

  const handlePause = useCallback(() => {
    stream.pause();
  }, [stream]);

  const handleSeek = useCallback(
    (time: number) => {
      setShowPlayButton(false);

      stream.seek(time);
    },
    [stream]
  );

  useImperativeHandle(ref, () => ({
    moveToTime: handleSeek,
  }));

  return (
    <Box height="100%" flex={1} p={3}>
      <Flex
        alignItems="stretch"
        flexDirection="column"
        border="1px solid"
        height="100%"
        borderColor="spotBackground.1"
        borderRadius={4}
        overflow="hidden"
        position="relative"
      >
        <CurrentEventInfo
          events={events}
          onSeek={handleSeek}
          ref={eventInfoRef}
        />

        {showPlayButton && (
          <PlayButton onClick={handlePlay}>
            <AdjustedPlay size="extra-large" />
          </PlayButton>
        )}

        <PlayPauseKeyboardShortcuts
          onPlay={handlePlay}
          onPause={handlePause}
          state={playerState}
        />

        <PlayerBox ref={playerRef} />

        <PlayerControls
          fullscreen={fullscreen}
          onToggleFullscreen={onToggleFullscreen}
          onToggleSidebar={onToggleSidebar}
          onToggleTimeline={onToggleTimeline}
          duration={duration}
          onPlay={handlePlay}
          onPause={handlePause}
          onSeek={handleSeek}
          state={playerState}
          ref={controlsRef}
        />
      </Flex>
    </Box>
  );
}

const PlayButton = styled.button`
  border: none;
  background: none;
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  background: rgba(0, 0, 0, 0.6);
  border-radius: 50%;
  padding: ${p => p.theme.space[3]}px;
  z-index: 10;
  cursor: pointer;

  &:hover {
    background: rgba(0, 0, 0, 0.4);
  }
`;

interface PlayPauseKeyboardShortcutsProps {
  onPlay: () => void;
  onPause: () => void;
  state: PlayerState;
}

function PlayPauseKeyboardShortcuts({
  onPlay,
  onPause,
  state,
}: PlayPauseKeyboardShortcutsProps) {
  const [visibleState, setVisibleState] = useState<
    PlayerState.Paused | PlayerState.Playing | null
  >(null);

  const timeoutRef = useRef<number | null>(null);

  useEffect(() => {
    if (!visibleState) {
      return;
    }

    if (timeoutRef.current) {
      window.clearTimeout(timeoutRef.current);
    }

    timeoutRef.current = window.setTimeout(() => {
      setVisibleState(null);
    }, 1000);

    return () => {
      if (timeoutRef.current) {
        window.clearTimeout(timeoutRef.current);
      }
    };
  }, [visibleState]);

  useEventListener('keydown', e => {
    if (e.code !== 'Space') {
      return;
    }

    e.preventDefault();

    if (timeoutRef.current) {
      window.clearTimeout(timeoutRef.current);
    }

    const next =
      state === PlayerState.Playing ? PlayerState.Paused : PlayerState.Playing;

    if (next === PlayerState.Playing) {
      onPlay();
    } else {
      onPause();
    }

    setVisibleState(next);
  });

  if (!visibleState) {
    return null;
  }

  return (
    <AnimatedState key={visibleState}>
      {visibleState === PlayerState.Playing && (
        <AdjustedPlay size="extra-large" />
      )}
      {visibleState === PlayerState.Paused && <Pause size="extra-large" />}
    </AnimatedState>
  );
}

const appear = keyframes`
  to {
    transform: translate(-50%, 50%) scale(2);
    opacity: 0;
  }
`;

// The play icon is slightly off center
const AdjustedPlay = styled(Play)`
  position: relative;
  left: -2px;
`;

const AnimatedState = styled.div`
  background: rgba(0, 0, 0, 0.8);
  border-radius: 50%;
  padding: ${p => p.theme.space[2]}px;
  bottom: 50%;
  color: white;
  display: flex;
  justify-content: center;
  left: 50%;
  position: absolute;
  transform: translate(-50%, 50%);
  z-index: 1;
  animation: ${appear} 0.8s linear forwards;
`;

const PlayerBox = styled.div`
  background: black; // black bars on the sides of the terminal
  flex: 1;
  position: relative;

  .xterm-viewport {
    overflow: hidden;
  }
`;
