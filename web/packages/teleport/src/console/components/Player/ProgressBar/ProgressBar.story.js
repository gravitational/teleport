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
import { storiesOf } from '@storybook/react';
import ProgressBar from './ProgressBar';

storiesOf('TeleportConsole/Player/ProgressBar', module)
  .add('ProgressBar (playing)', () => {
    return <SampleProgressbar isPlaying={true} />;
  })
  .add('ProgressBar (stopped)', () => {
    return <SampleProgressbar isPlaying={false} />;
  });

class SampleProgressbar extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      isPlaying: true,
      min: 1,
      max: 200,
      value: 1,
      time: '12:12',
      ...props,
    };
  }

  onChange = value => {
    this.setState({
      value,
    });
  };

  render() {
    const props = {
      ...this.state,
      onChange: this.onChange,
      onToggle: () => null,
    };

    return <ProgressBar {...props} />;
  }
}
