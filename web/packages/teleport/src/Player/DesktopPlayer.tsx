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

import React, { useEffect, useState } from 'react';
import styled from 'styled-components';
import { Indicator, Box, Alert } from 'design';
import useAttempt from 'shared/hooks/useAttemptNext';

import cfg from 'teleport/config';
import { PlayerClient, PlayerClientEvent } from 'teleport/lib/tdp';
import { getAccessToken, getHostName } from 'teleport/services/api';
import TdpClientCanvas from 'teleport/components/TdpClientCanvas';

import { ProgressBarDesktop } from './ProgressBar';

import type { PngFrame, ClientScreenSpec } from 'teleport/lib/tdp/codec';

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
    tdpCliOnWsClose,
    tdpCliOnTdpError,
    attempt,
  } = useDesktopPlayer({
    sid,
    clusterId,
  });

  const displayCanvas = attempt.status === 'success' || attempt.status === '';
  const displayProgressBar = attempt.status !== 'processing';

  return (
    <StyledPlayer>
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}

      {attempt.status === 'failed' && (
        <DesktopPlayerAlert my={4} mx={10} children={attempt.statusText} />
      )}

      <TdpClientCanvas
        tdpCli={playerClient}
        tdpCliInit={true}
        tdpCliOnPngFrame={tdpCliOnPngFrame}
        tdpCliOnClientScreenSpec={tdpCliOnClientScreenSpec}
        tdpCliOnWsClose={tdpCliOnWsClose}
        tdpCliOnTdpError={tdpCliOnTdpError}
        onContextMenu={() => true}
        // overflow: 'hidden' is needed to prevent the canvas from outgrowing the container due to some weird css flex idiosyncracy.
        // See https://gaurav5430.medium.com/css-flex-positioning-gotchas-child-expands-to-more-than-the-width-allowed-by-the-parent-799c37428dd6.
        style={{
          alignSelf: 'center',
          overflow: 'hidden',
          display: displayCanvas ? 'flex' : 'none',
        }}
      />
      <ProgressBarDesktop
        playerClient={playerClient}
        durationMs={durationMs}
        style={{
          display: displayProgressBar ? 'flex' : 'none',
        }}
        id="progressBarDesktop"
      />
    </StyledPlayer>
  );
};

const useDesktopPlayer = ({
  sid,
  clusterId,
}: {
  sid: string;
  clusterId: string;
}) => {
  const [playerClient, setPlayerClient] = useState<PlayerClient | null>(null);
  // attempt.status === '' means the playback ended gracefully
  const { attempt, setAttempt } = useAttempt('processing');

  useEffect(() => {
    setPlayerClient(
      new PlayerClient(
        cfg.api.desktopPlaybackWsAddr
          .replace(':fqdn', getHostName())
          .replace(':clusterId', clusterId)
          .replace(':sid', sid)
          .replace(':token', getAccessToken())
      )
    );
  }, [clusterId, sid]);

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

    setAttempt({ status: 'success' });
  };

  useEffect(() => {
    if (playerClient) {
      playerClient.addListener(PlayerClientEvent.SESSION_END, () => {
        setAttempt({ status: '' });
      });

      playerClient.addListener(
        PlayerClientEvent.PLAYBACK_ERROR,
        (err: Error) => {
          setAttempt({
            status: 'failed',
            statusText: `There was an error while playing this session: ${err.message}`,
          });
        }
      );

      return () => {
        playerClient.shutdown();
      };
    }
  }, [playerClient, setAttempt]);

  // If the websocket closed for some reason other than the session playback ending,
  // as signaled by the server (which sets prevAttempt.status = '' in
  // the PlayerClientEvent.SESSION_END event handler), or a TDP message from the server
  // signalling an error, assume some sort of network or playback error and alert the user.
  const tdpCliOnWsClose = () => {
    setAttempt(prevAttempt => {
      if (prevAttempt.status !== '' && prevAttempt.status !== 'failed') {
        return {
          status: 'failed',
          statusText: 'connection to the server failed for an unknown reason',
        };
      }
      return prevAttempt;
    });
  };

  const tdpCliOnTdpError = (error: Error) => {
    setAttempt({
      status: 'failed',
      statusText: error.message,
    });
  };

  return {
    playerClient,
    tdpCliOnPngFrame,
    tdpCliOnClientScreenSpec,
    tdpCliOnWsClose,
    tdpCliOnTdpError,
    attempt,
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
