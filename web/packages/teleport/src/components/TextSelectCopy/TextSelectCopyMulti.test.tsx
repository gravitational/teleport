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

import { render, screen, userEvent } from 'design/utils/testing';

import { TextSelectCopyMulti } from './TextSelectCopyMulti';

jest.useFakeTimers();

test('changing of icon when button is clicked', async () => {
  const user = userEvent.setup({ delay: null });
  render(<TextSelectCopyMulti lines={[{ text: 'some text to copy' }]} />);

  // Init button states.
  expect(screen.queryByTestId('btn-copy')).toBeVisible();
  expect(screen.queryByTestId('btn-check')).not.toBeVisible();

  // Clicking copy button should change the button icon to "check".
  await user.click(screen.getByTestId('btn-copy'));
  expect(screen.getByTestId('btn-check')).toBeVisible();
  expect(screen.getByTestId('btn-copy')).not.toBeVisible();

  const clipboardText = await navigator.clipboard.readText();
  expect(clipboardText).toBe('some text to copy');

  // After set time out, the buttons should return to its initial state.
  jest.runAllTimers();
  expect(screen.queryByTestId('btn-copy')).toBeVisible();
  expect(screen.queryByTestId('btn-check')).not.toBeVisible();
});

test('correct copying of texts', async () => {
  const user = userEvent.setup({ delay: null });
  render(
    <TextSelectCopyMulti
      lines={[
        { text: 'text to copy1', comment: 'first comment' },
        { text: 'text to copy2', comment: 'second comment' },
      ]}
    />
  );

  // Test copying of each text.
  const btns = screen.queryAllByRole('button');
  expect(btns).toHaveLength(2);

  await user.click(btns[1]);
  let clipboardText = await navigator.clipboard.readText();
  expect(clipboardText).toBe('text to copy2');

  await user.click(btns[0]);
  clipboardText = await navigator.clipboard.readText();
  expect(clipboardText).toBe('text to copy1');
});
