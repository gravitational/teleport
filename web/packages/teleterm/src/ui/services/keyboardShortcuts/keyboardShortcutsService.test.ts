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
