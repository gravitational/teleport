import { useEffect } from '@gravitational/shared/hooks';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import {
  KeyboardShortcutEventSubscriber,
  KeyboardShortcutHandlers,
} from './types';

export function useKeyboardShortcuts(handlers: KeyboardShortcutHandlers): void {
  const { keyboardShortcutsService: serviceKeyboardShortcuts } =
    useAppContext();

  useEffect(() => {
    const handleShortcutEvent: KeyboardShortcutEventSubscriber = event => {
      handlers[event.type]?.();
    };

    serviceKeyboardShortcuts.subscribeToEvents(handleShortcutEvent);
    return () =>
      serviceKeyboardShortcuts.unsubscribeFromEvents(handleShortcutEvent);
  }, [handlers]);
}
