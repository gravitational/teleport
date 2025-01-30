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

import { useCallback, useEffect, useMemo, useState } from 'react';
import styled from 'styled-components';

import { Alert, Box, Flex, Indicator } from 'design';

import TdpClientCanvas from 'teleport/components/TdpClientCanvas';
import cfg from 'teleport/config';
import { formatDisplayTime, StatusEnum } from 'teleport/lib/player';
import { PlayerClient, TdpClient } from 'teleport/lib/tdp';
import type { BitmapFrame } from 'teleport/lib/tdp/client';
import type { ClientScreenSpec, PngFrame } from 'teleport/lib/tdp/codec';
import { getHostName } from 'teleport/services/api';

import ProgressBar from './ProgressBar';

const reload = () => window.location.reload();
const handleContextMenu = () => true;
const PROGRESS_BAR_ID = 'progressBarDesktop';

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
    canvasSizeIsSet,

    clientOnPngFrame,
    clientOnBitmapFrame,
    clientOnClientScreenSpec,
    clientOnWsClose,
    clientOnTdpError,
    clientOnTdpInfo,
  } = useDesktopPlayer({
    sid,
    clusterId,
  });

  const isError = playerStatus === StatusEnum.ERROR || statusText !== '';
  const isLoading = playerStatus === StatusEnum.LOADING;
  const isPlaying = playerStatus === StatusEnum.PLAYING;
  const isComplete = isError || playerStatus === StatusEnum.COMPLETE;

  const t = isComplete
    ? durationMs // Force progress bar to 100% when playback is complete or errored.
    : time; // Otherwise, use the current time.

  // Hide the canvas and progress bar until the canvas' size has been fully defined.
  // This prevents visual glitches at pageload where the canvas starts out small and
  // then suddenly expands to its full size (moving the progress bar down with it).
  const canvasAndProgressBarDisplayStyle = canvasSizeIsSet
    ? {} // Canvas size is set, let TdpClientCanvas and ProgressBar use their default display styles.
    : { display: 'none' }; // Canvas size is not set, hide the canvas and progress bar.

  return (
    <StyledPlayer>
      {isError && <DesktopPlayerAlert my={4} children={statusText} />}
      {isLoading && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}

      <StyledContainer>
        {/* TODO (avatus) turn back on when types fixed */}
        {/* <TdpClientCanvas */}
        {/*   client={playerClient} */}
        {/*   clientShouldConnect={true} */}
        {/*   clientOnPngFrame={clientOnPngFrame} */}
        {/*   clientOnBmpFrame={clientOnBitmapFrame} */}
        {/*   clientOnClientScreenSpec={clientOnClientScreenSpec} */}
        {/*   clientOnWsClose={clientOnWsClose} */}
        {/*   clientOnTdpError={clientOnTdpError} */}
        {/*   clientOnTdpInfo={clientOnTdpInfo} */}
        {/*   canvasOnContextMenu={handleContextMenu} */}
        {/*   style={{ */}
        {/*     ...canvasStyle, */}
        {/*     ...canvasAndProgressBarDisplayStyle, */}
        {/*   }} */}
        {/* /> */}

        <ProgressBar
          id={PROGRESS_BAR_ID}
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
          style={{ ...canvasAndProgressBarDisplayStyle }}
        />
      </StyledContainer>
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

const useDesktopPlayer = ({ clusterId, sid }) => {
  const [time, setTime] = useState(0);
  const [playerStatus, setPlayerStatus] = useState(StatusEnum.LOADING);
  const [statusText, setStatusText] = useState('');
  // `canvasSizeIsSet` is used to track whether the canvas' size has been fully defined.
  const [canvasSizeIsSet, setCanvasSizeIsSet] = useState(false);

  const playerClient = useMemo(() => {
    const url = cfg.api.desktopPlaybackWsAddr
      .replace(':fqdn', getHostName())
      .replace(':clusterId', clusterId)
      .replace(':sid', sid);
    return new PlayerClient({ url, setTime, setPlayerStatus, setStatusText });
  }, [clusterId, sid]);

  const clientOnWsClose = useCallback(() => {
    if (playerClient) {
      playerClient.cancelTimeUpdate();
    }
  }, [playerClient]);

  const clientOnTdpError = useCallback((error: Error) => {
    setPlayerStatus(StatusEnum.ERROR);
    setStatusText(error.message || error.toString());
  }, []);

  const clientOnTdpInfo = useCallback((info: string) => {
    setPlayerStatus(StatusEnum.COMPLETE);
    setStatusText(info);
  }, []);

  const clientOnClientScreenSpec = useCallback(
    (_cli: TdpClient, canvas: HTMLCanvasElement, spec: ClientScreenSpec) => {
      const { width, height } = spec;

      const styledPlayer = canvas.parentElement;
      const progressBar = styledPlayer.children.namedItem(PROGRESS_BAR_ID);

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

      setCanvasSizeIsSet(true);
    },
    []
  );

  useEffect(() => {
    return () => {
      playerClient.shutdown();
    };
  }, [playerClient]);

  return {
    time,
    playerClient,
    playerStatus,
    statusText,
    canvasSizeIsSet,

    clientOnPngFrame,
    clientOnBitmapFrame,
    clientOnClientScreenSpec,
    clientOnWsClose,
    clientOnTdpError,
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
`;
