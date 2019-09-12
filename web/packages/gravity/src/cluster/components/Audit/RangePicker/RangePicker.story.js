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
import RangePicker from './RangePicker';

storiesOf('Gravity/Audit', module)
  .add('RangePicker', () => (
    <RangePicker {...props} />
  ))

const options = [
  {
    name: '7 days',
    from: new Date(),
    to: new Date(),
  },
  {
    name: 'Custom Range...',
    isCustom: true,
    from: new Date(),
    to: new Date(),
  }
]

const props = {
  disabled: false,
  value: options[0],
  options,
  onChange: () => {}
}