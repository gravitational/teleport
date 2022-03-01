import React, { useState, useEffect } from 'react';

import { throttle } from 'lodash';
import { dateToUtc } from 'shared/services/loc';
import { format } from 'date-fns';

import { PlayerClient, PlayerClientEvent } from 'teleport/lib/tdp';

import ProgressBar from './ProgressBar';

export const ProgressBarDesktop = (props: {
  playerClient: PlayerClient;
  durationMs: number;
  style?: React.CSSProperties;
  id?: string;
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
      style={props.style}
      id={props.id}
    />
  );
};
