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

import { ImmutableStore } from './immutableStore';

describe('subscribeWithSelector', () => {
  it('calls the callback only when a selected part of the state gets updated', () => {
    const store = new TestStore();

    const fooUpdatedCallback = jest.fn();
    store.subscribeWithSelector(state => state.foo, fooUpdatedCallback);

    const barUpdatedCallback = jest.fn();
    store.subscribeWithSelector(state => state.bar, barUpdatedCallback);

    store.setState(draft => {
      draft.foo.set('lorem', 'ipsum');
    });

    expect(fooUpdatedCallback).toHaveBeenCalledTimes(1);
    expect(barUpdatedCallback).not.toHaveBeenCalled();

    store.setState(draft => {
      draft.bar.set('dolor', 'sit');
    });

    expect(fooUpdatedCallback).toHaveBeenCalledTimes(1);
    expect(barUpdatedCallback).toHaveBeenCalledTimes(1);
  });

  it('returns a function which unsubscribes', () => {
    const store = new TestStore();

    const fooUpdatedCallback1 = jest.fn();
    store.subscribeWithSelector(state => state.foo, fooUpdatedCallback1);

    const fooUpdatedCallback2 = jest.fn();
    const unsubscribe = store.subscribeWithSelector(
      state => state.foo,
      fooUpdatedCallback2
    );
    unsubscribe();

    store.setState(draft => {
      draft.foo.set('lorem', 'ipsum');
    });

    expect(fooUpdatedCallback1).toHaveBeenCalledTimes(1);
    expect(fooUpdatedCallback2).not.toHaveBeenCalled();
  });

  it('calls the callbacks if multiple parts of the state get updated at the same time', () => {
    const store = new TestStore();

    const fooUpdatedCallback = jest.fn();
    store.subscribeWithSelector(state => state.foo, fooUpdatedCallback);

    const barUpdatedCallback = jest.fn();
    store.subscribeWithSelector(state => state.bar, barUpdatedCallback);

    const quuxUpdatedCallback = jest.fn();
    store.subscribeWithSelector(state => state.quux, quuxUpdatedCallback);

    store.setState(draft => {
      draft.foo.set('lorem', 'ipsum');
      draft.bar.set('dolor', 'sit');
    });

    expect(fooUpdatedCallback).toHaveBeenCalledTimes(1);
    expect(barUpdatedCallback).toHaveBeenCalledTimes(1);
    expect(quuxUpdatedCallback).not.toHaveBeenCalled();
  });

  it('calls the callbacks if a deeper part of the state gets updated', () => {
    const store = new TestStore();

    const bazUpdatedCallback = jest.fn();
    store.subscribeWithSelector(state => state.baz, bazUpdatedCallback);

    store.setState(draft => {
      // Update baz.items while the selector is set to just baz.
      draft.baz.items.set('lorem', 'ipsum');
    });

    expect(bazUpdatedCallback).toHaveBeenCalledTimes(1);
  });
});

class TestStore extends ImmutableStore<{
  foo: Map<string, string>;
  bar: Map<string, string>;
  quux: Map<string, string>;
  baz: {
    items: Map<string, string>;
  };
}> {
  constructor() {
    super();
    this.setState(() => ({
      foo: new Map(),
      bar: new Map(),
      quux: new Map(),
      baz: { items: new Map() },
    }));
  }
}
