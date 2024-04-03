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

import { waitFor } from '@testing-library/react';

import { EditBot } from 'teleport/Bots/EditBot';
import { EditBotProps } from 'teleport/Bots/types';

const makeProps = (overrides: Partial<EditBotProps> = {}): EditBotProps => ({
  fetchRoles: jest.fn().mockResolvedValueOnce([]),
  attempt: { status: '' },
  name: 'bot-007',
  onClose: () => {},
  onEdit: () => {},
  selectedRoles: [],
  setSelectedRoles: () => {},
  ...overrides,
});

test('renders', async () => {
  const props = makeProps({ selectedRoles: ['foo-role'] });
  render(<EditBot {...props} />);
  await waitFor(() => expect(props.fetchRoles).toHaveBeenCalledTimes(1));

  expect(screen.getByText('Edit Bot')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();

  expect(screen.getByRole('textbox', { name: 'Name' })).toHaveValue('bot-007');
  expect(screen.getByRole('textbox', { name: 'Name' })).toHaveAttribute(
    'readonly'
  );

  expect(screen.getByRole('textbox', { name: 'Bot Roles' })).toBeEnabled();
  expect(screen.getByText('foo-role')).toBeInTheDocument();
});

test('cancel calls onclose cb', async () => {
  const mockClose = jest.fn();
  const props = makeProps({ onClose: mockClose });
  render(<EditBot {...props} />);

  expect(mockClose).not.toHaveBeenCalled();
  await userEvent.click(screen.queryByRole('button', { name: 'Cancel' }));
  expect(mockClose).toHaveBeenCalled();
});

test('edit calls onedit cb', async () => {
  const mockEdit = jest.fn();
  const props = makeProps({ onEdit: mockEdit });
  render(<EditBot {...props} />);

  expect(mockEdit).not.toHaveBeenCalled();
  await userEvent.click(screen.queryByRole('button', { name: 'Save' }));
  expect(mockEdit).toHaveBeenCalled();
});

test('disables buttons when processing', async () => {
  const props = makeProps({ attempt: { status: 'processing' } });
  render(<EditBot {...props} />);
  await waitFor(() => expect(props.fetchRoles).toHaveBeenCalledTimes(1));

  expect(screen.queryByRole('button', { name: 'Save' })).toBeDisabled();
  expect(screen.queryByRole('button', { name: 'Cancel' })).toBeDisabled();
});

test('displays error text', async () => {
  const props = makeProps({
    attempt: { status: 'failed', statusText: 'error editing' },
  });
  render(<EditBot {...props} />);
  await waitFor(() => expect(props.fetchRoles).toHaveBeenCalledTimes(1));

  expect(screen.getByText('error editing')).toBeInTheDocument();
});
