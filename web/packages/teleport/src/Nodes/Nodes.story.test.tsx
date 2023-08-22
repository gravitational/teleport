/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { render } from 'design/utils/testing';

import { KeysEnum } from 'teleport/services/localStorage';

import { Loaded, Failed, Empty, EmptyReadOnly } from './Nodes.story';

// TODO (avatus) DELETE IN 15.0
// this is to allow the tests to actually render
// the correct tables
beforeAll(() => {
  localStorage.setItem(KeysEnum.UNIFIED_RESOURCES_DISABLED, 'true');
});

afterAll(() => {
  localStorage.removeItem(KeysEnum.UNIFIED_RESOURCES_DISABLED);
});

test('loaded', () => {
  const { container } = render(<Loaded />);
  expect(container.firstChild).toMatchSnapshot();
});

test('failed', () => {
  const { container } = render(<Failed />);
  expect(container.firstChild).toMatchSnapshot();
});

test('empty state', () => {
  const { container } = render(<Empty />);
  expect(container).toMatchSnapshot();
});

test('readonly empty state', () => {
  const { container } = render(<EmptyReadOnly />);
  expect(container).toMatchSnapshot();
});
