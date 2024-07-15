/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { render, fireEvent, screen } from 'design/utils/testing';

import {
  Default,
  Custom,
  Disabled,
  AtLeastOneRequired,
} from './LabelsInput.story';

test('defaults, with empty labels', async () => {
  render(<Default />);

  expect(screen.queryByText(/key/i)).not.toBeInTheDocument();
  expect(screen.queryByText(/value/i)).not.toBeInTheDocument();
  expect(screen.queryByText(/required field/i)).not.toBeInTheDocument();
  expect(screen.queryByPlaceholderText('label key')).not.toBeInTheDocument();
  expect(screen.queryByPlaceholderText('label value')).not.toBeInTheDocument();

  fireEvent.click(screen.getByText(/add a label/i));

  expect(screen.getByText(/key/i)).toBeInTheDocument();
  expect(screen.getByText(/value/i)).toBeInTheDocument();
  expect(screen.getAllByText(/required field/i)).toHaveLength(2);
  expect(screen.getByPlaceholderText('label key')).toBeInTheDocument();
  expect(screen.getByPlaceholderText('label value')).toBeInTheDocument();
  expect(screen.getByTitle(/remove label/i)).toBeInTheDocument();

  fireEvent.click(screen.getByText(/add another label/i));

  expect(screen.getAllByPlaceholderText('label key')).toHaveLength(2);
  expect(screen.getAllByPlaceholderText('label value')).toHaveLength(2);
  expect(screen.getAllByTitle(/remove label/i)).toHaveLength(2);
});

test('with custom texts', async () => {
  render(<Custom />);

  fireEvent.click(screen.getByText(/add a custom adjective/i));

  expect(screen.getByText(/custom key name/i)).toBeInTheDocument();
  expect(screen.getByText(/custom value/i)).toBeInTheDocument();
  expect(screen.getAllByText(/required field/i)).toHaveLength(2);
  expect(
    screen.getByPlaceholderText('custom key placeholder')
  ).toBeInTheDocument();
  expect(
    screen.getByPlaceholderText('custom value placeholder')
  ).toBeInTheDocument();

  expect(
    screen.getByRole('button', { name: 'Add another Custom Adjective' })
  ).toBeInTheDocument();

  // Delete the only row.
  fireEvent.click(screen.getByTitle(/remove custom adjective/i));
  expect(
    screen.getByRole('button', { name: 'Add a Custom Adjective' })
  ).toBeInTheDocument();
  expect(
    screen.queryByPlaceholderText('custom key placeholder')
  ).not.toBeInTheDocument();
  expect(
    screen.queryByPlaceholderText('custom value placeholder')
  ).not.toBeInTheDocument();
});

test('disabled buttons', async () => {
  render(<Disabled />);

  expect(screen.getByTitle(/remove label/i)).toBeDisabled();
  expect(
    screen.getByRole('button', { name: 'Add another Label' })
  ).toBeDisabled();
});

test('removing last label is not possible due to requiring labels', async () => {
  render(<AtLeastOneRequired />);

  expect(screen.getByPlaceholderText('label key')).toBeInTheDocument();
  expect(screen.getByPlaceholderText('label value')).toBeInTheDocument();

  fireEvent.click(screen.getByTitle(/remove label/i));

  expect(screen.getByPlaceholderText('label key')).toBeInTheDocument();
  expect(screen.getByPlaceholderText('label value')).toBeInTheDocument();
});
