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

import * as stories from './TextSelectCopyMulti.story';

test('render multi bash texts', () => {
  const { container } = render(<stories.BashMulti />);
  expect(container).toMatchSnapshot();
});

test('render multi bash texts with comment', () => {
  const { container } = render(<stories.BashMultiWithComment />);
  expect(container).toMatchSnapshot();
});

test('render single bash text', () => {
  const { container } = render(<stories.BashSingle />);
  expect(container).toMatchSnapshot();
});

test('render single bash text with comment', () => {
  const { container } = render(<stories.BashSingleWithComment />);
  expect(container).toMatchSnapshot();
});

test('render non bash single text', () => {
  const { container } = render(<stories.NonBash />);
  expect(container).toMatchSnapshot();
});
