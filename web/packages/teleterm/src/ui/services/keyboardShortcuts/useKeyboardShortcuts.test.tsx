import renderHook from 'design/utils/renderHook';

import React from 'react';

import AppContextProvider from 'teleterm/ui/appContextProvider';

import AppContext from 'teleterm/ui/appContext';

import { useKeyboardShortcuts } from './useKeyboardShortcuts';

import { KeyboardShortcutsService } from './keyboardShortcutsService';
import { KeyboardShortcutEventSubscriber } from './types';

test('call handler on its event type', () => {
  const { handler, getEventEmitter, wrapper } = getTestSetup();

  renderHook(() => useKeyboardShortcuts({ 'tab-1': handler }), { wrapper });
  const emitEvent = getEventEmitter();
  emitEvent({ type: 'tab-1' });

  expect(handler).toHaveBeenCalled();
});

test('do not call handler on other event type', () => {
  const { handler, getEventEmitter, wrapper } = getTestSetup();

  renderHook(() => useKeyboardShortcuts({ 'tab-1': handler }), { wrapper });
  const emitEvent = getEventEmitter();
  emitEvent({ type: 'tab-2' });

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
