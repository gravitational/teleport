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

import ProgressBar from './ProgressBar';

export default {
  title: 'Teleport/Player/ProgressBar',
};

export const Playing = () => {
  const [state, setState] = React.useState(() => ({
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
