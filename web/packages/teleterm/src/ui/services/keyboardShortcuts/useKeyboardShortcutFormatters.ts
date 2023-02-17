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
  const { mainProcessClient } = useAppContext();
  const { platform } = mainProcessClient.getRuntimeSettings();
  const { keyboardShortcuts } = mainProcessClient.configService.get();

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
