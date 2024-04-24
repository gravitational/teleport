/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React from 'react';

import { render, fireEvent, screen } from 'design/utils/testing';

import {
  SingleFlowInPlaceSlider,
  MultiFlowWheelSlider,
} from './StepSlider.story';

test('single flow', () => {
  render(<SingleFlowInPlaceSlider />);

  // Test initial render.
  expect(screen.getByText('Step 1')).toBeVisible();

  // Test going back when already at the beginning of array.
  // Should do nothing as expected.
  fireEvent.click(screen.getByText(/back1/i));
  expect(screen.getByText('Step 1')).toBeVisible();

  // Test next.
  fireEvent.click(screen.getByText(/next1/i));
  expect(screen.getByText('Step 2')).toBeVisible();

  // Test going next when at the end of array.
  // Should do nothing.
  fireEvent.click(screen.getByText(/next2/i));
  expect(screen.getByText('Step 2')).toBeVisible();

  // Test going back.
  fireEvent.click(screen.getByText(/back2/i));
  expect(screen.getByText('Step 1')).toBeVisible();
});

test('switching between multi flow', () => {
  render(<MultiFlowWheelSlider />);

  // Test initial primary flow.
  expect(screen.getByTestId('multi-primary1')).toBeVisible();

  // Test switching to secondary flow.
  fireEvent.click(screen.getByText(/secondary flow/i));
  expect(screen.getByTestId('multi-secondary1')).toBeVisible();

  // Test switching back to primary flow.
  fireEvent.click(screen.getByText(/primary flow/i));
  expect(screen.getByTestId('multi-primary1')).toBeVisible();
});

test('setting default step index', () => {
  render(<SingleFlowInPlaceSlider defaultStepIndex={1} />);

  expect(screen.getByText('Step 2')).toBeVisible();

  fireEvent.click(screen.getByText(/back2/i));
  expect(screen.getByText('Step 1')).toBeVisible();

  fireEvent.click(screen.getByText(/next1/i));
  expect(screen.getByText('Step 2')).toBeVisible();
});

test('setting default step index for multi flow', () => {
  render(<MultiFlowWheelSlider defaultStepIndex={1} />);

  expect(screen.getByTestId('multi-primary2')).toBeVisible();

  // Changing flows should reset the step to 0, not to the provided defaultStepIndex.
  fireEvent.click(screen.getByText(/secondary flow/i));
  expect(screen.getByTestId('multi-secondary1')).toBeVisible();
});
