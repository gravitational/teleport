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

import { renderHook } from '@testing-library/react';

import { act, render, screen } from 'design/utils/testing';

import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';

import { usePersistedState } from './usePersistedState';

it('propagates changes coming from the same usePersistedState invocation', () => {
  const appContext = new MockAppContext();
  render(
    <MockAppContextProvider appContext={appContext}>
      <Counter />
    </MockAppContextProvider>
  );

  act(() => {
    screen.getByText('Increase').click();
  });

  expect(screen.getByText('Counter: 1')).toBeInTheDocument();
  expect(appContext.statePersistenceService.getState()['counter']).toEqual(1);
});

it('reads initial state from persisted state', () => {
  const appContext = new MockAppContext();
  appContext.statePersistenceService.putState({ boolean: false } as any);

  render(
    <MockAppContextProvider appContext={appContext}>
      <Boolean />
    </MockAppContextProvider>
  );

  expect(screen.getByText('Boolean: false')).toBeInTheDocument();
});

it('updates only the given key', () => {
  const appContext = new MockAppContext();
  appContext.statePersistenceService.putState({ foo: 'bar' } as any);

  render(
    <MockAppContextProvider appContext={appContext}>
      <Counter />
    </MockAppContextProvider>
  );

  act(() => {
    screen.getByText('Increase').click();
  });

  expect(screen.getByText('Counter: 1')).toBeInTheDocument();
  expect(appContext.statePersistenceService.getState()['foo']).toEqual('bar');
});

// TODO(ravicious): Change the behavior of usePersistedState so it actually does propagate changes
// across callsites.
it('does not propagate changes across different usePersistedState invocations', () => {
  const appContext = new MockAppContext();
  render(
    <MockAppContextProvider appContext={appContext}>
      <Counter />
      <Counter />
    </MockAppContextProvider>
  );

  act(() => {
    screen.getAllByText('Increase')[0].click();
  });

  expect(screen.getByText('Counter: 1')).toBeInTheDocument();
  expect(screen.getByText('Counter: 0')).toBeInTheDocument();
  expect(appContext.statePersistenceService.getState()['counter']).toEqual(1);
});

it('accepts a function as an argument to setState which receives current state and properly handles undefined state', () => {
  const { result } = renderHook(
    () =>
      usePersistedState<'counters', { counters: { foo: number; bar: number } }>(
        'counters',
        { foo: 0, bar: 0 }
      ),
    { wrapper: MockAppContextProvider }
  );

  let [, setState] = result.current;
  act(() => {
    setState(currState => ({ ...currState, foo: currState.foo + 1 }));
  });
  [, setState] = result.current;
  act(() => {
    setState(currState => ({ ...currState, bar: currState.bar + 1 }));
  });

  let [state] = result.current;
  expect(state).toEqual({ foo: 1, bar: 1 });
});

it('properly handles existing state when given a function to setState', () => {
  const appContext = new MockAppContext();
  appContext.statePersistenceService.putState({
    counters: { foo: 2, bar: 2 },
  } as any);
  const { result } = renderHook(
    () =>
      usePersistedState<'counters', { counters: { foo: number; bar: number } }>(
        'counters',
        { foo: 0, bar: 0 }
      ),
    {
      wrapper: ({ children }) => (
        <MockAppContextProvider appContext={appContext}>
          {children}
        </MockAppContextProvider>
      ),
    }
  );

  let [, setState] = result.current;
  act(() => {
    setState(currState => ({ ...currState, foo: currState.foo + 1 }));
  });
  [, setState] = result.current;
  act(() => {
    setState(currState => ({ ...currState, bar: currState.bar - 1 }));
  });

  let [state] = result.current;
  expect(state).toEqual({ foo: 3, bar: 1 });
});

it('keeps the identity of the setter stable', () => {
  const { result } = renderHook(
    () =>
      usePersistedState<'counters', { counters: { foo: number } }>('counters', {
        foo: 1,
      }),
    { wrapper: MockAppContextProvider }
  );

  let [, setState] = result.current;
  const originalSetState = setState;
  act(() => {
    setState({ foo: 5 });
  });
  expect(result.current[1]).toBe(originalSetState);

  act(() => {
    setState(state => ({ ...state, foo: state.foo + 1 }));
  });
  expect(result.current[1]).toBe(originalSetState);
});

type TestState = { counter: number; boolean: boolean };

const Counter = () => {
  const [counter, setCounter] = usePersistedState<'counter', TestState>(
    'counter',
    0
  );

  return (
    <div>
      Counter: {counter}
      <button onClick={() => setCounter(counter + 1)}>Increase</button>
    </div>
  );
};

const Boolean = () => {
  const [boolean, setBoolean] = usePersistedState<'boolean', TestState>(
    'boolean',
    true
  );

  return (
    <div>
      Boolean: {boolean.toString()}
      <button onClick={() => setBoolean(!boolean)}>Toggle</button>
    </div>
  );
};
