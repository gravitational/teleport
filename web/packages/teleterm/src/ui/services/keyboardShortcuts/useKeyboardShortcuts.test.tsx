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

import renderHook from 'design/utils/renderHook';

import AppContext from 'teleterm/ui/appContext';
import AppContextProvider from 'teleterm/ui/appContextProvider';

import { KeyboardShortcutsService } from './keyboardShortcutsService';
import { KeyboardShortcutEventSubscriber } from './types';
import { useKeyboardShortcuts } from './useKeyboardShortcuts';

test('call handler on its event type', () => {
  const { handler, getEventEmitter, wrapper } = getTestSetup();

  renderHook(() => useKeyboardShortcuts({ tab1: handler }), { wrapper });
  const emitEvent = getEventEmitter();
  emitEvent({ action: 'tab1' });

  expect(handler).toHaveBeenCalled();
});

test('do not call handler on other event type', () => {
  const { handler, getEventEmitter, wrapper } = getTestSetup();

  renderHook(() => useKeyboardShortcuts({ tab1: handler }), { wrapper });
  const emitEvent = getEventEmitter();
  emitEvent({ action: 'tab2' });

  expect(handler).not.toHaveBeenCalled();
});

function getTestSetup() {
  const { getEventEmitter, wrapper } = makeWrapper();
  const handler = jest.fn();
  return { handler, getEventEmitter, wrapper };
}

function makeWrapper() {
  let eventEmitter: KeyboardShortcutEventSubscriber;
  return {
    wrapper: (props: any) => {
      const serviceKeyboardShortcuts: Partial<KeyboardShortcutsService> = {
        subscribeToEvents(subscriber: KeyboardShortcutEventSubscriber) {
          eventEmitter = subscriber;
        },
        unsubscribeFromEvents() {
          eventEmitter = null;
        },
      };

      return (
        <AppContextProvider
          value={
            { keyboardShortcutsService: serviceKeyboardShortcuts } as AppContext
          }
        >
          {props.children}
        </AppContextProvider>
      );
    },
    getEventEmitter: () => {
      return eventEmitter;
    },
  };
}
