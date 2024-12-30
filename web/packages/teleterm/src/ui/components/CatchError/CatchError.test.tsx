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

import { render, screen } from '@testing-library/react';

import '@testing-library/jest-dom';

import Logger, { NullService } from 'teleterm/logger';

import { CatchError } from './CatchError';

beforeAll(() => {
  Logger.init(new NullService());
});

beforeEach(() => {
  jest.restoreAllMocks();
  // Mock console.error, otherwise the test would output noise to the terminal.
  jest.spyOn(console, 'error').mockImplementation(() => {});
});

const ThrowError = (props: { error: any }) => {
  throw props.error;
};

it('renders caught error (without ThemeProvider being available)', () => {
  render(
    <CatchError>
      <ThrowError error={new Error('Lorem ipsum')} />
    </CatchError>
  );
  expect(screen.getByText('Lorem ipsum')).toBeInTheDocument();
  expect(console.error).toHaveBeenCalled();
});

it('works correctly when a non-Error value is thrown', () => {
  render(
    <CatchError>
      <ThrowError error={'Lorem ipsum'} />
    </CatchError>
  );
  expect(screen.getByText('Lorem ipsum')).toBeInTheDocument();
  expect(console.error).toHaveBeenCalled();
});
