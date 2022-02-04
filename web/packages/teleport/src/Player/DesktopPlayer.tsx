import React, { useState, useEffect } from 'react';
import styled from 'styled-components';
import ProgressBar from './ProgressBar';
import { PlayerClient, PlayerClientEvent } from 'teleport/lib/tdp';
import { PngFrame, ClientScreenSpec } from 'teleport/lib/tdp/codec';
import cfg from 'teleport/config';
import { getAccessToken, getHostName } from 'teleport/services/api';
import TdpClientCanvas from 'teleport/components/TdpClientCanvas';

export const DesktopPlayer = ({
  sid,
  clusterId,
}: {
  sid: string;
  clusterId: string;
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
      <ProgressBarDesktop playerClient={playerClient} />
    </StyledPlayer>
  );
};

export const ProgressBarDesktop = (props: { playerClient: PlayerClient }) => {
  const { playerClient } = props;

  const [state, setState] = useState({
    max: 500, // TODO(isaiah): total time of the recording in ms
    min: 0, // TODO(isaiah): the recording always starts at 0 ms
    current: 0, // TODO(isaiah): the current time the playback is at
    time: '-:--', // TODO(isaiah): the human readable time the playback is at
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

    return () => {
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
