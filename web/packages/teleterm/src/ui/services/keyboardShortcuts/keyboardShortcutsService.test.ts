import { ConfigService } from 'teleterm/services/config';

import { KeyboardShortcutsService } from './keyboardShortcutsService';

test('call subscriber on event', () => {
  const { subscriber } = getTestSetup();
  dispatchEventCommand1();
  expect(subscriber).toHaveBeenCalledWith({ type: 'tab-1' });
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

function getTestSetup() {
  const service = new KeyboardShortcutsService('darwin', {
    get: () => ({ keyboardShortcuts: { 'tab-1': 'Command-1' } }),
  } as unknown as ConfigService);
  const subscriber = jest.fn();
  service.subscribeToEvents(subscriber);
  return { service, subscriber };
}

function dispatchEventCommand1() {
  dispatchEvent(new KeyboardEvent('keydown', { metaKey: true, key: '1' }));
}
