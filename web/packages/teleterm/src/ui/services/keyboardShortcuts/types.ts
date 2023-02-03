import { KeyboardShortcutType } from 'teleterm/services/config';

export interface KeyboardShortcutEvent {
  type: KeyboardShortcutType;
}

export type KeyboardShortcutEventSubscriber = (
  event: KeyboardShortcutEvent
) => void;

export type KeyboardShortcutHandlers = Partial<
  Record<KeyboardShortcutType, () => void>
>;
