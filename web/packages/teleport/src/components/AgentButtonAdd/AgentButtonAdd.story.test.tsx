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

import {
  CanCreate,
  CannotCreate,
  CannotCreateVowel,
  OnLeaf,
  OnLeafVowel,
} from './AgentButtonAdd.story';

test('can create', () => {
  const { container } = render(<CanCreate />);
  expect(container).toMatchSnapshot();
});

test('cannot create', () => {
  const { container } = render(<CannotCreate />);
  expect(container).toMatchSnapshot();
});

test('cannot create when resource starts with vowel', () => {
  const { container } = render(<CannotCreateVowel />);
  expect(container).toMatchSnapshot();
});

test('on leaf cluster', () => {
  const { container } = render(<OnLeaf />);
  expect(container).toMatchSnapshot();
});

test('on leaf cluster when resource starts with vowel', () => {
  const { container } = render(<OnLeafVowel />);
  expect(container).toMatchSnapshot();
});
