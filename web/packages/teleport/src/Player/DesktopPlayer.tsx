import React, { useState, useEffect } from 'react';

import styled from 'styled-components';
import { throttle } from 'lodash';
import { dateToUtc } from 'shared/services/loc';
import { format } from 'date-fns';

import cfg from 'teleport/config';
import { PlayerClient, PlayerClientEvent } from 'teleport/lib/tdp';
import { PngFrame, ClientScreenSpec } from 'teleport/lib/tdp/codec';
import { getAccessToken, getHostName } from 'teleport/services/api';
import TdpClientCanvas from 'teleport/components/TdpClientCanvas';

import ProgressBar from './ProgressBar';

export const DesktopPlayer = ({
  sid,
  clusterId,
  durationMs,
}: {
  sid: string;
  clusterId: string;
  durationMs: number;
}) => {
  const { playerClient, tdpCliOnPngFrame, tdpCliOnClientScreenSpec } =
    useDesktopPlayer({
      sid,
      clusterId,
    });

  return (
    <StyledPlayer>
      <TdpClientCanvas
        tdpCli={playerClient}
        tdpCliOnPngFrame={tdpCliOnPngFrame}
        tdpCliOnClientScreenSpec={tdpCliOnClientScreenSpec}
        onContextMenu={() => true}
        // overflow: 'hidden' is needed to prevent the canvas from outgrowing the container due to some weird css flex idiosyncracy.
        // See https://gaurav5430.medium.com/css-flex-positioning-gotchas-child-expands-to-more-than-the-width-allowed-by-the-parent-799c37428dd6.
        style={{ display: 'flex', flexGrow: 1, overflow: 'hidden' }}
      />
      <ProgressBarDesktop playerClient={playerClient} durationMs={durationMs} />
    </StyledPlayer>
  );
};

export const ProgressBarDesktop = (props: {
  playerClient: PlayerClient;
  durationMs: number;
}) => {
  const { playerClient, durationMs } = props;

  const toHuman = (currentMs: number) => {
    return format(dateToUtc(new Date(currentMs)), 'mm:ss');
  };

  const [state, setState] = useState({
    max: durationMs,
    min: 0,
    current: 0, // the recording always starts at 0 ms
    time: toHuman(0),
    isPlaying: true, // determines whether play or pause symbol is shown
  });

  useEffect(() => {
    playerClient.addListener(PlayerClientEvent.TOGGLE_PLAY_PAUSE, () => {
      // setState({...state, isPlaying: !state.isPlaying}) doesn't work because
      // the listener is added when state == initialState, and that initialState
      // value is effectively hardcoded into its logic.
      setState(prevState => {
        return { ...prevState, isPlaying: !prevState.isPlaying };
      });
    });

    const throttledUpdateCurrentTime = throttle(
      currentTimeMs => {
        setState(prevState => {
          return {
            ...prevState,
            current: currentTimeMs,
            time: toHuman(currentTimeMs),
          };
        });
      },
      // Magic number to throttle progress bar updates so that the playback is smoother.
      50
    );

    playerClient.addListener(
      PlayerClientEvent.UPDATE_CURRENT_TIME,
      currentTimeMs => throttledUpdateCurrentTime(currentTimeMs)
    );

    playerClient.addListener(PlayerClientEvent.SESSION_END, () => {
      throttledUpdateCurrentTime.cancel();
      // TODO(isaiah): Make this smoother
      // https://github.com/gravitational/webapps/issues/579
      setState(prevState => {
        return { ...prevState, current: durationMs };
      });
    });

    return () => {
      throttledUpdateCurrentTime.cancel();
      playerClient.nuke();
    };
  }, [playerClient]);

  return (
    <ProgressBar
      {...state}
      toggle={() => playerClient.togglePlayPause()}
      move={() => {}}
    />
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
    canvas.width = spec.width;
    canvas.height = spec.height;
  };

  return {
    playerClient,
    tdpCliOnPngFrame,
    tdpCliOnClientScreenSpec,
  };
};

const StyledPlayer = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
`;
