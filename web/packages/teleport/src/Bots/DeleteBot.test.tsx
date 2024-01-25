/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { DeleteBot } from 'teleport/Bots/DeleteBot';
import { DeleteBotProps } from 'teleport/Bots/types';

const makeProps = (): DeleteBotProps => ({
  attempt: { status: '' },
  name: 'bot-007',
  onClose: () => {},
  onDelete: () => {},
});

test('renders', async () => {
  const props = makeProps();
  render(<DeleteBot {...props} />);

  expect(screen.getByText('Delete Bot?')).toBeInTheDocument();
  expect(
    screen.getByRole('button', { name: 'Yes, Delete Bot' })
  ).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();

  screen.getByText((content, node) => {
    const hasText = node =>
      node.textContent === 'Are you sure you want to delete Bot bot-007 ?';
    const nodeHasText = hasText(node);
    const childrenDontHaveText = Array.from(node.children).every(
      child => !hasText(child)
    );
    return nodeHasText && childrenDontHaveText;
  });
});

test('cancel calls onclose cb', async () => {
  const props = makeProps();
  const mockClose = jest.fn();
  props.onClose = mockClose;
  render(<DeleteBot {...props} />);

  expect(mockClose).not.toHaveBeenCalled();
  await userEvent.click(screen.queryByRole('button', { name: 'Cancel' }));
  expect(mockClose).toHaveBeenCalled();
});

test('delete calls ondelete cb', async () => {
  const props = makeProps();
  const mockDelete = jest.fn();
  props.onDelete = mockDelete;
  render(<DeleteBot {...props} />);

  expect(mockDelete).not.toHaveBeenCalled();
  await userEvent.click(
    screen.queryByRole('button', { name: 'Yes, Delete Bot' })
  );
  expect(mockDelete).toHaveBeenCalled();
});

test('disables buttons when processing', async () => {
  const props = makeProps();
  props.attempt = { status: 'processing' };
  render(<DeleteBot {...props} />);

  expect(
    screen.queryByRole('button', { name: 'Yes, Delete Bot' })
  ).toBeDisabled();
  expect(screen.queryByRole('button', { name: 'Cancel' })).toBeDisabled();
});

test('displays error text', async () => {
  const props = makeProps();
  props.attempt = { status: 'failed', statusText: 'error deleting' };
  render(<DeleteBot {...props} />);

  expect(screen.getByText('error deleting')).toBeInTheDocument();
});
