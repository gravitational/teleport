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

import { render, screen } from 'design/utils/testing';

import * as story from './UserTokenLink.story';

jest
  .spyOn(Date, 'now')
  .mockImplementation(() => Date.parse('2021-04-08T07:00:00Z'));

test('reset link dialog as invite', () => {
  render(<story.Invite />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('reset link dialog', () => {
  render(<story.Reset />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});
