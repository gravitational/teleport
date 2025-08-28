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

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import styled from 'styled-components';

import { Alert, Box, Flex, Indicator } from 'design';
import {
  CanvasRenderer,
  CanvasRendererRef,
} from 'shared/components/CanvasRenderer';
import { useListener } from 'shared/libs/tdp';

import cfg from 'teleport/config';
import { formatDisplayTime, StatusEnum } from 'teleport/lib/player';
import { PlayerClient } from 'teleport/lib/tdp';
import { getHostName } from 'teleport/services/api';

import ProgressBar from './ProgressBar';

const reload = () => window.location.reload();

export const DesktopPlayer = ({
  sid,
  clusterId,
  durationMs,
}: {
  sid: string;
  clusterId: string;
  durationMs: number;
}) => {
  const {
    playerClient,
    playerStatus,
    statusText,
    time,

    clientOnTransportOpen,
    clientOnTransportClose,
    clientOnError,
    clientOnTdpInfo,
  } = useDesktopPlayer({
    sid,
    clusterId,
  });
  const canvasRendererRef = useRef<CanvasRendererRef>(null);

  useListener(playerClient?.onError, clientOnError);
  useListener(playerClient?.onInfo, clientOnTdpInfo);
  useListener(playerClient?.onTransportOpen, clientOnTransportOpen);
  useListener(playerClient?.onTransportClose, clientOnTransportClose);
  useListener(
    playerClient?.onPngFrame,
    canvasRendererRef.current?.renderPngFrame
  );
  useListener(
    playerClient?.onBmpFrame,
    canvasRendererRef.current?.renderBitmapFrame
  );
  useListener(
    playerClient?.onScreenSpec,
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

        <ProgressBar
          min={0}
          max={durationMs}
          current={t}
          disabled={isComplete}
          isPlaying={isPlaying}
          time={formatDisplayTime(t)}
          onRestart={reload}
          onStartMove={() => playerClient.suspendTimeUpdates()}
          move={pos => {
            playerClient.resumeTimeUpdates();
            playerClient.seekTo(pos);
          }}
          onPlaySpeedChange={s => playerClient.setPlaySpeed(s)}
          toggle={() => playerClient.togglePlayPause()}
        />
      </StyledContainer>
    </StyledPlayer>
  );
};

const useDesktopPlayer = ({ clusterId, sid }) => {
  const [time, setTime] = useState(0);
  const [playerStatus, setPlayerStatus] = useState(StatusEnum.LOADING);
  const [statusText, setStatusText] = useState('');

  const playerClient = useMemo(() => {
    const url = cfg.api.desktopPlaybackWsAddr
      .replace(':fqdn', getHostName())
      .replace(':clusterId', clusterId)
      .replace(':sid', sid);
    return new PlayerClient({ url, setTime, setPlayerStatus, setStatusText });
  }, [clusterId, sid]);

  const clientOnTransportOpen = useCallback(() => {
    setPlayerStatus(StatusEnum.PLAYING);
  }, []);

  const clientOnTransportClose = useCallback(() => {
    if (playerClient) {
      playerClient.cancelTimeUpdate();
    }
  }, [playerClient]);

  const clientOnError = useCallback((error: Error) => {
    setPlayerStatus(StatusEnum.ERROR);
    setStatusText(error.message);
  }, []);

  const clientOnTdpInfo = useCallback((info: string) => {
    setPlayerStatus(StatusEnum.COMPLETE);
    setStatusText(info);
  }, []);

  useEffect(() => {
    if (!playerClient) {
      return;
    }
    void playerClient.connect();
    return () => {
      playerClient.shutdown();
    };
  }, [playerClient]);

  return {
    time,
    playerClient,
    playerStatus,
    statusText,

    clientOnTransportOpen,
    clientOnTransportClose,
    clientOnError,
    clientOnTdpInfo,
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
  flex-direction: column;
  justify-content: center;
  width: 100%;
  height: 100%;
  min-height: 0;
`;
