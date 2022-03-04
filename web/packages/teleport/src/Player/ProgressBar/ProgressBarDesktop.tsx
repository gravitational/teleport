import React, { useState, useEffect, useRef } from 'react';

import { throttle } from 'lodash';
import { dateToUtc } from 'shared/services/loc';
import { format } from 'date-fns';

import {
  PlayerClient,
  PlayerClientEvent,
  TdpClientEvent,
} from 'teleport/lib/tdp';

import ProgressBar from './ProgressBar';

export const ProgressBarDesktop = (props: {
  playerClient: PlayerClient;
  durationMs: number;
  style?: React.CSSProperties;
  id?: string;
}) => {
  const { playerClient, durationMs } = props;
  const intervalRef = useRef<NodeJS.Timer>();

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

  // updateCurrentTime is a helper function to update the state variable.
  // It should be used within a setState, like
  // setState(prevState => {
  //   return updateCurrentTime(prevState, newTime)
  // })
  const updateCurrentTime = (
    prevState: typeof state,
    currentTimeMs: number
  ) => {
    return {
      ...prevState,
      current: currentTimeMs,
      time: toHuman(currentTimeMs),
    };
  };

  useEffect(() => {
    if (playerClient) {
      // Starts the smoothing interval, which smooths out the progress of the progress bar.
      // This ensures the bar continues to progress even during playbacks where there are long
      // intervals between TDP events sent to us by the server. The interval should be active
      // whenever the playback is in "play" mode.
      const smoothOutProgress = () => {
        const smoothingInterval = 25;

        intervalRef.current = setInterval(() => {
          setState(prevState => {
            const nextTimeMs = prevState.current + smoothingInterval;
            if (nextTimeMs <= durationMs) {
              return updateCurrentTime(prevState, nextTimeMs);
            } else {
              stopProgress();
              return updateCurrentTime(prevState, durationMs);
            }
          });
        }, smoothingInterval);
      };

      // The player always starts in play mode, so call this initially.
      smoothOutProgress();

      // Clears the smoothing interval and cancels any throttled updates,
      // should be called when the playback is paused or ended.
      const stopProgress = () => {
        throttledUpdateCurrentTime.cancel();
        clearInterval(intervalRef.current);
      };

      const throttledUpdateCurrentTime = throttle(
        currentTimeMs => {
          setState(prevState => {
            return updateCurrentTime(prevState, currentTimeMs);
          });
        },
        // Magic number to throttle progress bar updates caused by TDP events
        //  so that the playback is smoother.
        50
      );

      // Listens for UPDATE_CURRENT_TIME events which coincide with
      // TDP events sent to the playerClient by the server.
      playerClient.addListener(
        PlayerClientEvent.UPDATE_CURRENT_TIME,
        currentTimeMs => throttledUpdateCurrentTime(currentTimeMs)
      );

      playerClient.addListener(PlayerClientEvent.TOGGLE_PLAY_PAUSE, () => {
        // setState({...state, isPlaying: !state.isPlaying}) doesn't work because
        // the listener is added when state == initialState, and that initialState
        // value is effectively hardcoded into its logic.
        setState(prevState => {
          if (prevState.isPlaying) {
            // pause
            stopProgress();
          } else {
            // play
            smoothOutProgress();
          }
          return { ...prevState, isPlaying: !prevState.isPlaying };
        });
      });

      return () => {
        playerClient.nuke();
        stopProgress();
      };
    }
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
