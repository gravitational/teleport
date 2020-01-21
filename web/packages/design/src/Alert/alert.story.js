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
import { withInfo } from '@storybook/addon-info';
import Alert from './index';

storiesOf('Design/Alert', module)
  .addDecorator(withInfo)
  .add('Alert Component', () => {
    return <Alert>This is an error message</Alert>;
  })
  .add('Danger Alert', () => {
    return <Alert kind="danger">This is an error message</Alert>;
  })
  .add('Warning Alert', () => {
    return <Alert kind="warning">This is a warning message</Alert>;
  })
  .add('Info Alert', () => {
    return <Alert kind="info">This is a informational message</Alert>;
  })
  .add('Success Alert', () => {
    return <Alert kind="success">This is a success message</Alert>;
  });
