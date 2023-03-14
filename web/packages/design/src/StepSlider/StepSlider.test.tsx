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

import { render, fireEvent, screen } from 'design/utils/testing';

import { SingleStaticFlow, MultiCardFlow } from './StepSlider.story';

test('single flow', () => {
  render(<SingleStaticFlow />);

  // Test initial render.
  expect(screen.getByTestId('single-body1')).toBeVisible();

  // Test going back when already at the beginning of array.
  // Should do nothing as expected.
  fireEvent.click(screen.getByText(/back1/i));
  expect(screen.getByTestId('single-body1')).toBeVisible();

  // Test next.
  fireEvent.click(screen.getByText(/next1/i));
  expect(screen.getByTestId('single-body2')).toBeVisible();

  // Test going next when at the end of array.
  // Should do nothing.
  fireEvent.click(screen.getByText(/next2/i));
  expect(screen.getByTestId('single-body2')).toBeVisible();

  // Test going back.
  fireEvent.click(screen.getByText(/back2/i));
  expect(screen.getByTestId('single-body1')).toBeVisible();
});

test('switching between multi flow', () => {
  render(<MultiCardFlow />);

  // Test initial primary flow.
  expect(screen.getByTestId('multi-primary1')).toBeVisible();

  // Test switching to secondary flow.
  fireEvent.click(screen.getByText(/secondary flow/i));
  expect(screen.getByTestId('multi-secondary1')).toBeVisible();

  // Test switching back to primary flow.
  fireEvent.click(screen.getByText(/primary flow/i));
  expect(screen.getByTestId('multi-primary1')).toBeVisible();
});
