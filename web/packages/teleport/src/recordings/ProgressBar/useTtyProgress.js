/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { throttle } from 'lodash';

export default function useTtyProgress(tty) {
  const [state, rerender] = React.useState(() => {
    const playerState = makePlayerState(tty);

    function onTogglePlayStop() {
      if (tty.isPlaying()) {
        tty.stop();
      } else {
        tty.play();
      }
    }

    function onMove(value) {
      tty.move(value);
    }

    return {
      ...playerState,
      onTogglePlayStop,
      onMove,
    };
  });

  React.useEffect(() => {
    function onChange() {
      const newState = makePlayerState(tty);
      rerender({
        ...state,
        ...newState,
      });
    }

    const throttledOnChange = throttle(onChange, 500);

    function cleanup() {
      throttledOnChange.cancel();
      tty.stop();
      tty.removeAllListeners();
    }

    tty.on('change', throttledOnChange);

    return cleanup;
  }, [tty]);

  return state;
}

function makePlayerState(tty) {
  return {
    eventCount: tty.getEventCount(),
    duration: tty.duration,
    min: 1,
    time: tty.getCurrentTime(),
    isLoading: tty.isLoading(),
    isPlaying: tty.isPlaying(),
    isError: tty.isError(),
    errText: tty.errText,
    current: tty.current,
    canPlay: tty.length > 1,
  };
}
