/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { ComponentProps, PropsWithChildren } from 'react';

import { Providers, render, screen, userEvent } from 'design/utils/testing';

import InputSearch from './InputSearch';

describe('InputSearch', () => {
  test('renders', async () => {
    renderComponent({
      searchValue: '',
      setSearchValue: jest.fn(),
    });

    expect(screen.getByPlaceholderText('Search...')).toBeInTheDocument();
  });

  test('submits a search', async () => {
    const setSearchValue = jest.fn();

    const { user } = renderComponent({
      searchValue: '',
      setSearchValue,
    });

    const input = screen.getByPlaceholderText('Search...');
    await user.click(input);
    await user.paste('Lorem ipsum delor sit amet.');
    await user.type(input, '{enter}');

    expect(setSearchValue).toHaveBeenCalledTimes(1);
    expect(setSearchValue).toHaveBeenLastCalledWith(
      'Lorem ipsum delor sit amet.'
    );
  });
});

function renderComponent(props: ComponentProps<typeof InputSearch>) {
  const user = userEvent.setup();
  return {
    ...render(<InputSearch {...props} />, { wrapper: makeWrapper() }),
    user,
  };
}

function makeWrapper() {
  return (props: PropsWithChildren) => {
    return <Providers>{props.children}</Providers>;
  };
}
