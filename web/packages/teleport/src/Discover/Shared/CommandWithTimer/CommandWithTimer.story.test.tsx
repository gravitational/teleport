/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { render } from 'design/utils/testing';

import * as stories from './CommandWithTimer.story';

test('render default polling', async () => {
  const { container } = render(<stories.DefaultPolling />);
  expect(container).toMatchSnapshot();
});

test('render default polling success', async () => {
  const { container } = render(<stories.DefaultPollingSuccess />);
  expect(container).toMatchSnapshot();
});

test('render default polling error', async () => {
  const { container } = render(<stories.DefaultPollingError />);
  expect(container).toMatchSnapshot();
});

test('render custom polling', async () => {
  const { container } = render(<stories.CustomPolling />);
  expect(container).toMatchSnapshot();
});

test('render custom polling success', async () => {
  const { container } = render(<stories.CustomPollingSuccess />);
  expect(container).toMatchSnapshot();
});

test('render custom polling error', async () => {
  const { container } = render(<stories.CustomPollingError />);
  expect(container).toMatchSnapshot();
});
