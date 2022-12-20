import { KeyboardShortcutType } from '../../../services/config';
import { useAppContext } from '../../appContextProvider';
import { Platform } from '../../../mainProcess/types';

interface KeyboardShortcutFormatters {
  getLabelWithShortcut(
    label: string,
    shortcutKey: KeyboardShortcutType,
    options?: KeyboardShortcutFormattingOptions
  ): string;

  getShortcut(
    shortcutKey: KeyboardShortcutType,
    options?: KeyboardShortcutFormattingOptions
  ): string;
}

interface KeyboardShortcutFormattingOptions {
  useWhitespaceSeparator?: boolean;
}

export function useKeyboardShortcutFormatters(): KeyboardShortcutFormatters {
  const { mainProcessClient, keyboardShortcutsService } = useAppContext();
  const { platform } = mainProcessClient.getRuntimeSettings();
  const keyboardShortcuts = keyboardShortcutsService.getShortcutsConfig();

  return {
    getLabelWithShortcut(label, shortcutKey, options) {
      const formattedShortcut = formatKeyboardShortcut({
        platform,
        shortcutValue: keyboardShortcuts[shortcutKey],
        ...options,
      });
      return `${label} (${formattedShortcut})`;
    },
    getShortcut(shortcutKey, options) {
      return formatKeyboardShortcut({
        platform,
        shortcutValue: keyboardShortcuts[shortcutKey],
        ...options,
      });
    },
  };
}

function formatKeyboardShortcut(options: {
  platform: Platform;
  shortcutValue: string;
  useWhitespaceSeparator?: boolean;
}): string {
  switch (options.platform) {
    case 'darwin':
      return options.shortcutValue
        .replace('-', options.useWhitespaceSeparator ? ' ' : '')
        .replace('Command', '⌘')
        .replace('Control', '⌃')
        .replace('Option', '⌥')
        .replace('Shift', '⇧');
    default:
      return options.shortcutValue.replace(
        '-',
        options.useWhitespaceSeparator ? ' + ' : '+'
      );
  }
}
