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

import { useState } from 'react';

import ProgressBar from './ProgressBar';

export default {
  title: 'Teleport/Player/ProgressBar',
};

export const Playing = () => {
  const [state, setState] = useState(() => ({
    isPlaying: true,
    current: 100,
    min: 1,
    max: 200,
    value: 1,
    time: '12:12',
  }));

  function move(current) {
    setState({
      ...state,
      current: current,
    });
  }

  function toggle() {
    setState({
      ...state,
      isPlaying: !state.isPlaying,
    });
  }

  const props = {
    ...state,
    move,
    toggle,
  };

  return <ProgressBar {...props} />;
};
