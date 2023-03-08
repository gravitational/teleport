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

import { createMockConfigService } from 'teleterm/services/config/fixtures/mocks';

import { KeyboardShortcutsService } from './keyboardShortcutsService';

test('call subscriber on event', () => {
  const { subscriber } = getTestSetup();
  dispatchEventCommand1();
  expect(subscriber).toHaveBeenCalledWith({ action: 'tab1' });
});

test('do not call subscriber on unknown event', () => {
  const { subscriber } = getTestSetup();
  dispatchEvent(
    new KeyboardEvent('keydown', { metaKey: true, altKey: true, key: 'M' })
  );
  expect(subscriber).not.toHaveBeenCalled();
});

test('do not call subscriber after it has been unsubscribed', () => {
  const { service, subscriber } = getTestSetup();
  service.unsubscribeFromEvents(subscriber);
  dispatchEvent(new KeyboardEvent('keydown', { metaKey: true, key: '1' }));
  expect(subscriber).not.toHaveBeenCalled();
});

test('duplicate accelerators are returned', () => {
  const service = new KeyboardShortcutsService(
    'darwin',
    createMockConfigService({
      'keymap.tab1': 'Command+1',
      'keymap.tab2': 'Command+1',
      'keymap.tab3': 'Command+2',
    })
  );

  expect(service.getDuplicateAccelerators()).toStrictEqual({
    'Command+1': ['tab1', 'tab2'],
  });
});

function getTestSetup() {
  const service = new KeyboardShortcutsService(
    'darwin',
    createMockConfigService({ 'keymap.tab1': 'Command+1' })
  );
  const subscriber = jest.fn();
  service.subscribeToEvents(subscriber);
  return { service, subscriber };
}

function dispatchEventCommand1() {
  dispatchEvent(new KeyboardEvent('keydown', { metaKey: true, code: '1' }));
}
