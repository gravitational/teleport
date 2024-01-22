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

import React from 'react';
import styled from 'styled-components';
import { Indicator, Box, Alert } from 'design';

import cfg from 'teleport/config';
import { StatusEnum } from 'teleport/lib/player';
import { PlayerClient, TdpClient } from 'teleport/lib/tdp';
import { getAccessToken, getHostName } from 'teleport/services/api';
import TdpClientCanvas from 'teleport/components/TdpClientCanvas';

import { formatDisplayTime } from 'teleport/lib/player';

import ProgressBar from './ProgressBar';

import type { PngFrame, ClientScreenSpec } from 'teleport/lib/tdp/codec';
import type { BitmapFrame } from 'teleport/lib/tdp/client';

const reload = () => window.location.reload();
const handleContextMenu = () => true;

// overflow: 'hidden' is needed to prevent the canvas from outgrowing the container due to some weird css flex idiosyncracy.
// See https://gaurav5430.medium.com/css-flex-positioning-gotchas-child-expands-to-more-than-the-width-allowed-by-the-parent-799c37428dd6.
const canvasStyle = {
  alignSelf: 'center',
  overflow: 'hidden',
};

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

    clientOnPngFrame,
    clientOnBitmapFrame,
    clientOnClientScreenSpec,
    clientOnWsClose,
    clientOnTdpError,
  } = useDesktopPlayer({
    sid,
    clusterId,
  });

  const isError = playerStatus === StatusEnum.ERROR;
  const isLoading = playerStatus === StatusEnum.LOADING;
  const isPlaying = playerStatus === StatusEnum.PLAYING;
  const isComplete = isError || playerStatus === StatusEnum.COMPLETE;

  const t = playerStatus === StatusEnum.COMPLETE ? durationMs : time;

  return (
    <StyledPlayer>
      {isError && <DesktopPlayerAlert my={4} mx={10} children={statusText} />}
      {isLoading && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      <TdpClientCanvas
        client={playerClient}
        clientShouldConnect={true}
        clientOnPngFrame={clientOnPngFrame}
        clientOnBmpFrame={clientOnBitmapFrame}
        clientOnClientScreenSpec={clientOnClientScreenSpec}
        clientOnWsClose={clientOnWsClose}
        clientOnTdpError={clientOnTdpError}
        canvasOnContextMenu={handleContextMenu}
        style={canvasStyle}
      />

      {/* TODO(zmb3): why need lambda here? */}
      <ProgressBar
        id="progressBarDesktop"
        min={0}
        max={durationMs}
        current={t}
        disabled={isComplete}
        isPlaying={isPlaying}
        time={formatDisplayTime(t)}
        onRestart={reload}
        onStartMove={() => playerClient.suspendTimeUpdates()}
        move={pos => {
          playerClient.seekTo(pos);
          playerClient.resumeTimeUpdates();
        }}
        onPlaySpeedChange={s => playerClient.setPlaySpeed(s)}
        toggle={() => playerClient.togglePlayPause()}
      />
    </StyledPlayer>
  );
};

const clientOnPngFrame = (
  ctx: CanvasRenderingContext2D,
  pngFrame: PngFrame
) => {
  ctx.drawImage(pngFrame.data, pngFrame.left, pngFrame.top);
};

const clientOnBitmapFrame = (
  ctx: CanvasRenderingContext2D,
  bmpFrame: BitmapFrame
) => {
  ctx.putImageData(bmpFrame.image_data, bmpFrame.left, bmpFrame.top);
};

const clientOnClientScreenSpec = (
  cli: TdpClient,
  canvas: HTMLCanvasElement,
  spec: ClientScreenSpec
) => {
  const { width, height } = spec;

  const styledPlayer = canvas.parentElement;
  const progressBar = styledPlayer.children.namedItem('progressBarDesktop');

  const fullWidth = styledPlayer.clientWidth;
  const fullHeight = styledPlayer.clientHeight - progressBar.clientHeight;
  const originalAspectRatio = width / height;
  const currentAspectRatio = fullWidth / fullHeight;

  if (originalAspectRatio > currentAspectRatio) {
    // Use the full width of the screen and scale the height.
    canvas.style.height = `${(fullWidth * height) / width}px`;
  } else if (originalAspectRatio < currentAspectRatio) {
    // Use the full height of the screen and scale the width.
    canvas.style.width = `${(fullHeight * width) / height}px`;
  }

  canvas.width = width;
  canvas.height = height;
};

const useDesktopPlayer = ({ clusterId, sid }) => {
  const [time, setTime] = React.useState(0);
  const [playerStatus, setPlayerStatus] = React.useState(StatusEnum.LOADING);
  const [statusText, setStatusText] = React.useState('');

  const playerClient = React.useMemo(() => {
    const url = cfg.api.desktopPlaybackWsAddr
      .replace(':fqdn', getHostName())
      .replace(':clusterId', clusterId)
      .replace(':sid', sid)
      .replace(':token', getAccessToken());
    return new PlayerClient({ url, setTime, setPlayerStatus, setStatusText });
  }, [clusterId, sid, setTime, setPlayerStatus]);

  const clientOnWsClose = React.useCallback(() => {
    if (playerClient) {
      playerClient.cancelTimeUpdate();
    }
  }, [playerClient]);

  const clientOnTdpError = React.useCallback(
    (error: Error) => {
      setPlayerStatus(StatusEnum.ERROR);
      setStatusText(error.message || error.toString());
    },
    [setPlayerStatus, setStatusText]
  );

  React.useEffect(() => {
    return playerClient.shutdown;
  }, [playerClient]);

  return {
    time,
    playerClient,
    playerStatus,
    statusText,

    clientOnPngFrame,
    clientOnBitmapFrame,
    clientOnClientScreenSpec,
    clientOnWsClose,
    clientOnTdpError,
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
  align-self: center;
  min-width: 450px;

  // Overrides StyledPlayer container's justify-content
  // https://stackoverflow.com/a/34063808/6277051
  margin-bottom: auto;
`;
