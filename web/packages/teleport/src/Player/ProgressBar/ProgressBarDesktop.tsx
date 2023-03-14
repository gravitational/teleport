/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState, useEffect, useRef } from 'react';

import { throttle } from 'shared/utils/highbar';
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
  const intervalRef = useRef<NodeJS.Timer>();
  let playSpeed = 1.0;

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
      const smoothOutProgress = (speed: number) => {
        const smoothingInterval = 25;

        intervalRef.current = setInterval(() => {
          setState(prevState => {
            const nextTimeMs = prevState.current + smoothingInterval * speed;
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
      smoothOutProgress(playSpeed);

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
            smoothOutProgress(playSpeed);
          }
          return { ...prevState, isPlaying: !prevState.isPlaying };
        });
      });

      playerClient.addListener(
        PlayerClientEvent.PLAY_SPEED,
        (speed: number) => {
          playSpeed = speed;

          setState(prevState => {
            if (prevState.isPlaying) {
              stopProgress();
              smoothOutProgress(playSpeed);
            }
            return { ...prevState, isPlaying: prevState.isPlaying };
          });
        }
      );

      return () => {
        playerClient.shutdown();
        stopProgress();
      };
    }
  }, [playerClient]);

  return (
    <ProgressBar
      {...state}
      toggle={() => playerClient.togglePlayPause()}
      onPlaySpeedChange={(newSpeed: number) =>
        playerClient.setPlaySpeed(newSpeed)
      }
      move={() => {}}
      style={props.style}
      id={props.id}
      onRestart={() => window.location.reload()}
    />
  );
};
