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
