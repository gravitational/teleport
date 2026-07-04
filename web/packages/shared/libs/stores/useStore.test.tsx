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

import { act, render, screen } from 'design/utils/testing';

import Store from './store';
import useStore from './useStore';

test('components subscribes to store changes and unsubscribes on unmount', async () => {
  const store = new Store();

  store.setState({
    firstname: 'bob',
    lastname: 'smith',
  });

  const { unmount } = render(<Component store={store} />);

  expect(screen.getByTestId('state')).toHaveTextContent(
    JSON.stringify(store.state)
  );

  act(() => {
    store.setState({
      firstname: 'alex',
    });
  });

  expect(screen.getByTestId('state')).toHaveTextContent(
    JSON.stringify({
      firstname: 'alex',
      lastname: 'smith',
    })
  );

  jest.spyOn(store, 'unsubscribe');
  unmount();
  expect(store.unsubscribe).toHaveBeenCalledTimes(1);
});

function Component({ store }) {
  // subscribes to store updates
  useStore(store);
  return <span data-testid="state">{JSON.stringify(store.state)}</span>;
}
