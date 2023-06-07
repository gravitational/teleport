/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import renderHook from 'design/utils/renderHook';

import React from 'react';

import AppContextProvider from 'teleterm/ui/appContextProvider';

import AppContext from 'teleterm/ui/appContext';

import { useKeyboardShortcuts } from './useKeyboardShortcuts';

import { KeyboardShortcutsService } from './keyboardShortcutsService';
import { KeyboardShortcutEventSubscriber } from './types';

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
