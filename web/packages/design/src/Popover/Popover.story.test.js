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
import { Sample, Tooltip } from './Popover.story';
import { render, fireEvent } from 'design/utils/testing';

test('onClick popovers renders', () => {
  const { getByTestId, getByText, queryByTestId } = render(<Sample />);

  fireEvent.click(getByText(/left/i));
  expect(getByTestId('content')).toBeInTheDocument();
  fireEvent.click(getByTestId('backdrop'));
  expect(queryByTestId('content')).not.toBeInTheDocument();
});

test('onMouse tooltip render', () => {
  const { getByTestId, queryByTestId } = render(<Tooltip />);

  fireEvent.mouseOver(getByTestId('text'));
  expect(getByTestId('content')).toBeInTheDocument();

  fireEvent.mouseOut(getByTestId('text'));
  expect(queryByTestId('content')).not.toBeInTheDocument();
});
