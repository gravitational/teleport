/**
 * Copyright 2020 Gravitational, Inc.
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
import { screen } from '@testing-library/react';

import { render, fireEvent } from 'design/utils/testing';

import { Sample, Tooltip } from './Popover.story';

test('onClick popovers renders', () => {
  render(<Sample />);

  fireEvent.click(screen.getByText('Left'));
  expect(screen.getByTestId('content')).toBeInTheDocument();
  fireEvent.click(screen.getByTestId('backdrop'));
  expect(screen.queryByTestId('content')).not.toBeInTheDocument();
});

test('onMouse tooltip render', () => {
  render(<Tooltip />);

  fireEvent.mouseOver(screen.getByTestId('text'));
  expect(screen.getByTestId('content')).toBeInTheDocument();

  fireEvent.mouseOut(screen.getByTestId('text'));
  expect(screen.queryByTestId('content')).not.toBeInTheDocument();
});
