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
import { throttle } from 'shared/utils/highbar';

import TtyPlayer from 'teleport/lib/term/ttyPlayer';

import ProgressBar from './ProgressBar';

export default function ProgressBarTty(props: { tty: TtyPlayer }) {
  const state = useTtyProgress(props.tty);
  return <ProgressBar {...state} />;
}

export function useTtyProgress(tty: TtyPlayer) {
  const [state, setState] = React.useState(() => {
    return makeTtyProgress(tty);
  });

  React.useEffect(() => {
    const throttledOnChange = throttle(
      onChange,
      // some magic numbers to reduce number of re-renders when
      // session is too long and "eventful"
      Math.max(Math.min(tty.duration * 0.025, 500), 20)
    );

    function onChange() {
      // recalculate progress state
      const ttyProgres = makeTtyProgress(tty);
      setState(ttyProgres);
    }

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

function makeTtyProgress(tty: TtyPlayer) {
  function toggle() {
    if (tty.isPlaying()) {
      tty.stop();
    } else {
      tty.play();
    }
  }

  function move(value) {
    tty.move(value);
  }

  return {
    max: tty.duration,
    min: 1,
    time: tty.getCurrentTime(),
    isLoading: tty.isLoading(),
    isPlaying: tty.isPlaying(),
    current: tty.current,
    move,
    toggle,
  };
}
