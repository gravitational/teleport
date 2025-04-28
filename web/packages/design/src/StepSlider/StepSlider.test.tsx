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

import {
  fireEvent,
  render,
  screen,
  waitForElementToBeRemoved,
} from 'design/utils/testing';

import {
  MultiFlowWheelSlider,
  SingleFlowInPlaceSlider,
} from './StepSlider.story';

test('single flow', async () => {
  // Use custom animation duration to make tests faster.
  render(<SingleFlowInPlaceSlider tDuration={0} />);

  // Test initial render.
  expect(screen.getByTestId('single-body1')).toBeVisible();

  // Test going back when already at the beginning of array.
  // Should do nothing as expected.
  fireEvent.click(screen.getByText(/back1/i));
  expect(screen.getByTestId('single-body1')).toBeVisible();
  // The above is not enough; make sure we didn't start transitioning.
  expect(screen.queryByTestId('single-body2')).not.toBeInTheDocument();

  // Test next.
  fireEvent.click(screen.getByText(/next1/i));
  expect(screen.getByTestId('single-body2')).toBeVisible();
  await waitForElementToBeRemoved(() => screen.queryByTestId('single-body1'));

  // Test going next when at the end of array.
  // Should do nothing.
  fireEvent.click(screen.getByText(/next2/i));
  expect(screen.getByTestId('single-body2')).toBeVisible();
  // The above is not enough; make sure we didn't start transitioning.
  expect(screen.queryByTestId('single-body1')).not.toBeInTheDocument();

  // Test going back.
  fireEvent.click(screen.getByText(/back2/i));
  expect(screen.getByTestId('single-body1')).toBeVisible();
});

test('single flow with wrapping', async () => {
  // Use custom animation duration to make tests faster.
  render(<SingleFlowInPlaceSlider wrapping tDuration={0} />);
  expect(screen.getByTestId('single-body1')).toBeVisible();

  // Test going backwards on step 1
  fireEvent.click(screen.getByText(/back1/i));
  expect(screen.getByTestId('single-body3')).toBeVisible();
  await waitForElementToBeRemoved(() => screen.queryByTestId('single-body1'));

  // Test going forwards on step 3
  fireEvent.click(screen.getByText(/next3/i));
  expect(screen.getByTestId('single-body1')).toBeVisible();
  await waitForElementToBeRemoved(() => screen.queryByTestId('single-body3'));

  // Test the "normal" flow: forwards on step 1...
  fireEvent.click(screen.getByText(/next1/i));
  expect(screen.getByTestId('single-body2')).toBeVisible();
  await waitForElementToBeRemoved(() => screen.queryByTestId('single-body1'));

  // ...and backwards on step 2.
  fireEvent.click(screen.getByText(/back2/i));
  expect(screen.getByTestId('single-body1')).toBeVisible();
  await waitForElementToBeRemoved(() => screen.queryByTestId('single-body2'));
});

test('switching between multi flow', async () => {
  // Use custom animation duration to make tests faster.
  render(<MultiFlowWheelSlider tDuration={0} />);

  // Test initial primary flow.
  expect(screen.getByTestId('multi-primary1')).toBeVisible();

  // Test switching to secondary flow.
  fireEvent.click(screen.getByText(/secondary flow/i));
  expect(screen.getByTestId('multi-secondary1')).toBeVisible();
  await waitForElementToBeRemoved(() => screen.queryByTestId('multi-primary1'));

  // Test switching back to primary flow.
  fireEvent.click(screen.getByText(/primary flow/i));
  expect(screen.getByTestId('multi-primary1')).toBeVisible();
});
