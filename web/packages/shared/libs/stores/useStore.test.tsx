/**
 * Copyright 2020 Gravitational, Inc.
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

import { act, render } from 'design/utils/testing';

import useStore from './useStore';
import Store from './store';

test('components subscribes to store changes and unsubscribes on unmount', async () => {
  const store = new Store();

  store.setState({
    firstname: 'bob',
    lastname: 'smith',
  });

  const { unmount, container } = render(<Component store={store} />);

  expect(container.innerHTML).toBe(JSON.stringify(store.state));

  act(() => {
    store.setState({
      firstname: 'alex',
    });
  });

  expect(container.innerHTML).toBe(
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
  return <>{JSON.stringify(store.state)}</>;
}
