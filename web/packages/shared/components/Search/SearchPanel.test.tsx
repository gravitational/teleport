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

import { SearchPanel } from './SearchPanel';

describe('SearchPanel', () => {
  test('renders', async () => {
    renderComponent({
      filter: {
        search: '',
      },
      updateSearch: jest.fn(),
      updateQuery: jest.fn(),
      disableSearch: false,
    });

    expect(screen.getByPlaceholderText('Search...')).toBeInTheDocument();
  });

  test('submits a search', async () => {
    const updateSearch = jest.fn();
    const updateQuery = jest.fn();

    const { user } = renderComponent({
      filter: {
        search: '',
      },
      updateSearch,
      updateQuery,
      hideAdvancedSearch: true,
      disableSearch: false,
    });

    const input = screen.getByPlaceholderText('Search...');
    await user.click(input);
    await user.paste('Lorem ipsum delor sit amet.');
    await user.type(input, '{enter}');

    expect(updateSearch).toHaveBeenCalledTimes(1);
    expect(updateSearch).toHaveBeenLastCalledWith(
      'Lorem ipsum delor sit amet.'
    );
    expect(updateQuery).not.toHaveBeenCalled();
  });

  test('submits a query (advanced)', async () => {
    const updateSearch = jest.fn();
    const updateQuery = jest.fn();

    const { user } = renderComponent({
      filter: {
        query: '',
      },
      updateSearch,
      updateQuery,
      hideAdvancedSearch: false,
      disableSearch: false,
    });

    // Toggle advanced mode on
    await userEvent.click(screen.getByLabelText('Advanced'));

    const input = screen.getByPlaceholderText('Search...');
    await user.click(input);
    await user.paste('Lorem ipsum delor sit amet.');
    await user.type(input, '{enter}');

    expect(updateQuery).toHaveBeenCalledTimes(1);
    expect(updateQuery).toHaveBeenLastCalledWith('Lorem ipsum delor sit amet.');
    expect(updateSearch).not.toHaveBeenCalled();
  });
});

function renderComponent(props: ComponentProps<typeof SearchPanel>) {
  const user = userEvent.setup();
  return {
    ...render(<SearchPanel {...props} />, { wrapper: makeWrapper() }),
    user,
  };
}

function makeWrapper() {
  return (props: PropsWithChildren) => {
    return <Providers>{props.children}</Providers>;
  };
}
