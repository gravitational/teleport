import React, { useState } from 'react';

import styled from 'styled-components';

import { Indicator, Box } from 'design';

import cfg from 'teleport/config';
import { PlayerClient } from 'teleport/lib/tdp';
import { PngFrame, ClientScreenSpec } from 'teleport/lib/tdp/codec';
import { getAccessToken, getHostName } from 'teleport/services/api';
import TdpClientCanvas from 'teleport/components/TdpClientCanvas';

import { ProgressBarDesktop } from './ProgressBar';

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
    tdpCliOnPngFrame,
    tdpCliOnClientScreenSpec,
    showCanvas,
  } = useDesktopPlayer({
    sid,
    clusterId,
  });

  return (
    <>
      <StyledPlayer>
        {!showCanvas && (
          <Box textAlign="center" m={10}>
            <Indicator />
          </Box>
        )}

        <TdpClientCanvas
          tdpCli={playerClient}
          tdpCliOnPngFrame={tdpCliOnPngFrame}
          tdpCliOnClientScreenSpec={tdpCliOnClientScreenSpec}
          onContextMenu={() => true}
          // overflow: 'hidden' is needed to prevent the canvas from outgrowing the container due to some weird css flex idiosyncracy.
          // See https://gaurav5430.medium.com/css-flex-positioning-gotchas-child-expands-to-more-than-the-width-allowed-by-the-parent-799c37428dd6.
          style={{
            alignSelf: 'center',
            overflow: 'hidden',
            display: showCanvas ? 'flex' : 'none',
          }}
        />
        <ProgressBarDesktop
          playerClient={playerClient}
          durationMs={durationMs}
          style={{
            display: showCanvas ? 'flex' : 'none',
          }}
          id="progressBarDesktop"
        />
      </StyledPlayer>
    </>
  );
};

const useDesktopPlayer = ({
  sid,
  clusterId,
}: {
  sid: string;
  clusterId: string;
}) => {
  const playerClient = new PlayerClient(
    cfg.api.desktopPlaybackWsAddr
      .replace(':fqdn', getHostName())
      .replace(':clusterId', clusterId)
      .replace(':sid', sid)
      .replace(':token', getAccessToken())
  );

  const [showCanvas, setShowCanvas] = useState(false);

  const tdpCliOnPngFrame = (
    ctx: CanvasRenderingContext2D,
    pngFrame: PngFrame
  ) => {
    ctx.drawImage(pngFrame.data, pngFrame.left, pngFrame.top);
  };

  const tdpCliOnClientScreenSpec = (
    canvas: HTMLCanvasElement,
    spec: ClientScreenSpec
  ) => {
    const styledPlayer = canvas.parentElement;
    const progressBar = styledPlayer.children.namedItem('progressBarDesktop');

    const fullWidth = styledPlayer.clientWidth;
    const fullHeight = styledPlayer.clientHeight - progressBar.clientHeight;
    const originalAspectRatio = spec.width / spec.height;
    const currentAspectRatio = fullWidth / fullHeight;

    if (originalAspectRatio > currentAspectRatio) {
      // Use the full width of the screen and scale the height.
      canvas.style.height = `${(fullWidth * spec.height) / spec.width}px`;
    } else if (originalAspectRatio < currentAspectRatio) {
      // Use the full height of the screen and scale the width.
      canvas.style.width = `${(fullHeight * spec.width) / spec.height}px`;
    }

    canvas.width = spec.width;
    canvas.height = spec.height;

    setShowCanvas(true);
  };

  return {
    playerClient,
    tdpCliOnPngFrame,
    tdpCliOnClientScreenSpec,
    showCanvas,
  };
};

const StyledPlayer = styled.div`
  display: flex;
  flex-direction: column;
  justify-content: center;
  width: 100%;
  height: 100%;
`;
