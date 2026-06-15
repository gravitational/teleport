/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
  type Ref,
  type RefObject,
} from 'react';
import styled from 'styled-components';

import { Alert, Box, Flex, Indicator } from 'design';
import {
  CanvasRenderer,
  CanvasRendererRef,
} from 'shared/components/CanvasRenderer';
import { useListener } from 'shared/libs/tdp';

import cfg from 'teleport/config';
import { formatDisplayTime, StatusEnum } from 'teleport/lib/player';
import { PlayerClient, type PlayerTimeAnchor } from 'teleport/lib/tdp';
import { getHostName } from 'teleport/services/api';
import type { SessionRecordingEvent } from 'teleport/services/recordings';

import {
  CurrentEventInfo,
  type CurrentEventInfoHandle,
} from './CurrentEventInfo';
import ProgressBar from './ProgressBar';
import type { PlayerHandle } from './SshPlayer';

const reload = () => window.location.reload();

// how often the React-rendered ProgressBar is updated (instead of every animation frame)
const PROGRESS_UPDATE_INTERVAL_MS = 50;

interface DesktopPlayerProps {
  clusterId: string;
  durationMs: number;
  /** Enables the current event overlay (e.g. the skip inactivity button). */
  events?: SessionRecordingEvent[];
  onTimeChange?: (time: number) => void;
  ref?: Ref<PlayerHandle>;
  sid: string;
}

export const DesktopPlayer = ({
  sid,
  clusterId,
  durationMs,
  events,
  onTimeChange,
  ref,
}: DesktopPlayerProps) => {
  const canvasRendererRef = useRef<CanvasRendererRef>(null);
  const eventInfoRef = useRef<CurrentEventInfoHandle>(null);

  const {
    playerClient,
    playerStatus,
    statusText,
    time,

    seekTo,
    setPlaySpeed,
    suspendProgressUpdates,
    togglePlayPause,
  } = useDesktopPlayer({
    sid,
    clusterId,
    durationMs,
    eventInfoRef,
    onTimeChange,
  });

  useImperativeHandle(ref, () => ({ moveToTime: seekTo }), [seekTo]);

  useListener(
    playerClient.onPngFrame,
    canvasRendererRef.current?.renderPngFrame
  );
  useListener(
    playerClient.onBmpFrame,
    canvasRendererRef.current?.renderBitmapFrame
  );
  useListener(
    playerClient.onScreenSpec,
    canvasRendererRef.current?.setResolution
  );

  const isError = playerStatus === StatusEnum.ERROR || statusText !== '';
  const isLoading = playerStatus === StatusEnum.LOADING;
  const isPlaying = playerStatus === StatusEnum.PLAYING;
  const isComplete = isError || playerStatus === StatusEnum.COMPLETE;

  const t = isComplete
    ? durationMs // Force progress bar to 100% when playback is complete or errored.
    : time; // Otherwise, use the current time.

  return (
    <StyledPlayer>
      {isError && <DesktopPlayerAlert my={4}>{statusText}</DesktopPlayerAlert>}
      {isLoading && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}

      <StyledContainer>
        <CanvasRenderer ref={canvasRendererRef} />

        {events && (
          <CurrentEventInfo
            events={events}
            onSeek={seekTo}
            ref={eventInfoRef}
          />
        )}

        <ProgressBar
          min={0}
          max={durationMs}
          current={t}
          disabled={isComplete}
          isPlaying={isPlaying}
          time={formatDisplayTime(t)}
          onRestart={reload}
          onStartMove={suspendProgressUpdates}
          move={seekTo}
          onPlaySpeedChange={setPlaySpeed}
          toggle={togglePlayPause}
        />
      </StyledContainer>
    </StyledPlayer>
  );
};

interface TimeAnchor extends PlayerTimeAnchor {
  receivedAt: number;
}

const useDesktopPlayer = ({
  clusterId,
  durationMs,
  eventInfoRef,
  onTimeChange,
  sid,
}: {
  clusterId: string;
  durationMs: number;
  eventInfoRef: RefObject<CurrentEventInfoHandle | null>;
  onTimeChange?: (time: number) => void;
  sid: string;
}) => {
  const [time, setTime] = useState(0);
  const [playerStatus, setPlayerStatus] = useState(StatusEnum.LOADING);
  const [statusText, setStatusText] = useState('');

  // latest authoritative playback position, interpolated between by the requestAnimationFrame loop
  const anchorRef = useRef<TimeAnchor>({
    ms: 0,
    speed: 1,
    paused: true,
    receivedAt: 0,
  });
  // interpolated time as of the last frame, used to rebase on pause/play/speed changes
  const currentTimeRef = useRef(0);
  const lastProgressUpdateRef = useRef(0);
  // progress bar updates are suspended while the user drags the slider
  const progressSuspendedRef = useRef(false);

  const playerClient = useMemo(() => {
    const url = cfg.api.desktopPlaybackWsAddr
      .replace(':fqdn', getHostName())
      .replace(':clusterId', clusterId)
      .replace(':sid', sid);
    return new PlayerClient({ url });
  }, [clusterId, sid]);

  const rebaseAnchor = useCallback((changes: Partial<PlayerTimeAnchor>) => {
    anchorRef.current = {
      ...anchorRef.current,
      ms: currentTimeRef.current,
      receivedAt: performance.now(),
      ...changes,
    };
  }, []);

  const handleTimeUpdate = useCallback((anchor: PlayerTimeAnchor) => {
    anchorRef.current = { ...anchor, receivedAt: performance.now() };
  }, []);

  const handlePlayerStatus = useCallback(
    (status: StatusEnum) => {
      setPlayerStatus(status);

      if (status === StatusEnum.PLAYING || status === StatusEnum.PAUSED) {
        rebaseAnchor({ paused: status === StatusEnum.PAUSED });
      }
    },
    [rebaseAnchor]
  );

  const clientOnTransportOpen = useCallback(() => {
    setPlayerStatus(StatusEnum.PLAYING);
    anchorRef.current = {
      ms: 0,
      speed: anchorRef.current.speed,
      paused: false,
      receivedAt: performance.now(),
    };
  }, []);

  const clientOnTransportClose = useCallback(() => {
    // freeze the clock, there will be no more authoritative positions
    rebaseAnchor({ paused: true });
  }, [rebaseAnchor]);

  const clientOnError = useCallback((error: Error) => {
    setPlayerStatus(StatusEnum.ERROR);
    setStatusText(error.message);
  }, []);

  const clientOnTdpInfo = useCallback((info: string) => {
    setPlayerStatus(StatusEnum.COMPLETE);
    setStatusText(info);
  }, []);

  useListener(playerClient.onTimeUpdate, handleTimeUpdate);
  useListener(playerClient.onPlayerStatus, handlePlayerStatus);
  useListener(playerClient.onError, clientOnError);
  useListener(playerClient.onInfo, clientOnTdpInfo);
  useListener(playerClient.onTransportOpen, clientOnTransportOpen);
  useListener(playerClient.onTransportClose, clientOnTransportClose);

  const seekTo = useCallback(
    (pos: number) => {
      progressSuspendedRef.current = false;
      playerClient.seekTo(pos);
      setTime(pos);
    },
    [playerClient]
  );

  const setPlaySpeed = useCallback(
    (speed: number) => {
      rebaseAnchor({ speed });
      playerClient.setPlaySpeed(speed);
    },
    [playerClient, rebaseAnchor]
  );

  const suspendProgressUpdates = useCallback(() => {
    progressSuspendedRef.current = true;
  }, []);

  const togglePlayPause = useCallback(() => {
    playerClient.togglePlayPause();
  }, [playerClient]);

  useEffect(() => {
    let handle = 0;

    const update = () => {
      const { ms, speed, paused, receivedAt } = anchorRef.current;
      const now = performance.now();
      const current = Math.min(
        paused ? ms : ms + (now - receivedAt) * speed,
        durationMs
      );

      currentTimeRef.current = current;

      onTimeChange?.(current);
      eventInfoRef.current?.setTime(current);

      if (
        !progressSuspendedRef.current &&
        now - lastProgressUpdateRef.current >= PROGRESS_UPDATE_INTERVAL_MS
      ) {
        lastProgressUpdateRef.current = now;
        setTime(current);
      }

      handle = requestAnimationFrame(update);
    };

    handle = requestAnimationFrame(update);

    return () => cancelAnimationFrame(handle);
  }, [durationMs, onTimeChange, eventInfoRef]);

  useEffect(() => {
    playerClient.connect().catch(clientOnError);
    return () => {
      playerClient.shutdown();
    };
  }, [playerClient, clientOnError]);

  return {
    time,
    playerClient,
    playerStatus,
    statusText,

    seekTo,
    setPlaySpeed,
    suspendProgressUpdates,
    togglePlayPause,
  };
};

const StyledPlayer = styled.div`
  display: flex;
  flex-direction: column;
  justify-content: center;
  width: 100%;
  height: 100%;
`;

const DesktopPlayerAlert = styled(Alert)`
  position: absolute;
  top: 0;
  align-self: center;
  min-width: 450px;
`;

const StyledContainer = styled(Flex)`
  position: relative;
  flex-direction: column;
  justify-content: center;
  width: 100%;
  height: 100%;
  min-height: 0;
`;
