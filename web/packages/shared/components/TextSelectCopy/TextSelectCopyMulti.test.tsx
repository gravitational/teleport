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
