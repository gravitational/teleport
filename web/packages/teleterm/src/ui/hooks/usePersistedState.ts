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

import { Dispatch, SetStateAction, useCallback, useRef, useState } from 'react';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { StatePersistenceState } from 'teleterm/ui/services/statePersistence';

/**
 * usePersistedState is like useState, but it persists the state to app_state.json under the given
 * key.
 *
 * IMPORTANT: Currently, usePersistedState doesn't propagate changes across several callsites. That
 * is, if two callsites use the same key, calling setState in one component will not cause the other
 * component to update.
 *
 * This will _not_ work as expected:
 *
 * const Counter = () => {
 *   const [count, setCount] = usePersistedState('count', 0);
 *
 *   return (
 *     <div>
 *       {count}
 *       <button onClick={() => setCount(count + 1)}>Increase</button>
 *     </div>
 *   );
 * }
 *
 * () => {
 *   return (
 *     <>
 *       <Counter />
 *       <Counter />
 *     </>
 *   );
 * }
 *
 * However, this will work as expected:
 *
 * @example
 * const Counter = ({count, setCount}) => {
 *   return (
 *     <div>
 *       {count}
 *       <button onClick={() => setCount(count + 1)}>Increase</button>
 *     </div>
 *   );
 * }
 *
 * () => {
 *   const [count, setCount] = usePersistedState('count', 0);
 *
 *   return (
 *     <>
 *       <Counter count={count} setCount={setCount} />
 *       <Counter count={count} setCount={setCount} />
 *     </>
 *   );
 * }
 */
export function usePersistedState<
  // key could've been any string, but in lieu of avoiding typos, it's better to take it
  // from one central definition.
  Key extends keyof WholeState,
  // WholeState is purely for testing purposes to replace StatePersistenceState in tests.
  WholeState extends object = StatePersistenceState,
>(
  key: Key,
  initialState: WholeState[Key]
): [WholeState[Key], Dispatch<SetStateAction<WholeState[Key]>>] {
  const { statePersistenceService } = useAppContext();

  // setState below must be stable across updates, so it can't depend on initialState nor
  // wholeState. Hence the shenanigans with initialStateRef and getState.
  const initialStateRef = useRef(initialState);
  const getState = useCallback(() => {
    const wholeState = statePersistenceService.getState() as WholeState;
    const state = Object.hasOwn(wholeState, key)
      ? wholeState[key]
      : initialStateRef.current;
    return { wholeState, state };
  }, [key, statePersistenceService]);

  // TODO(ravicious): usePersistedState currently doesn't propagate changes across several
  // callsites.
  //
  // usePersistedState should either use useSyncExternalStore or some other solution to register a
  // listener in statePersistenceService that gets called whenever the given key gets updated.
  const [, rerender] = useState<object>();

  const setState: Dispatch<SetStateAction<WholeState[Key]>> = useCallback(
    newState => {
      const { wholeState, state } = getState();

      if (typeof newState === 'function') {
        statePersistenceService.putState({
          ...(wholeState as StatePersistenceState),
          [key]: (newState as (prevState: WholeState[Key]) => WholeState[Key])(
            state
          ),
        });
      } else {
        statePersistenceService.putState({
          ...(wholeState as StatePersistenceState),
          [key]: newState,
        });
      }

      rerender({});
    },
    [key, statePersistenceService, getState]
  );

  return [getState().state, setState];
}
