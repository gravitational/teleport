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

import { Platform } from '../../../mainProcess/types';
import { KeyboardShortcutAction } from '../../../services/config';
import { useAppContext } from '../../appContextProvider';

interface KeyboardShortcutFormatters {
  getLabelWithAccelerator(
    label: string,
    action: KeyboardShortcutAction,
    options?: KeyboardShortcutFormattingOptions
  ): string;

  getAccelerator(
    action: KeyboardShortcutAction,
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
    getLabelWithAccelerator(label, action, options) {
      const formattedAccelerator = formatAccelerator({
        platform,
        accelerator: keyboardShortcuts[action],
        ...options,
      });
      return `${label} (${formattedAccelerator})`;
    },
    getAccelerator(action, options) {
      return formatAccelerator({
        platform,
        accelerator: keyboardShortcuts[action],
        ...options,
      });
    },
  };
}

function formatAccelerator(options: {
  platform: Platform;
  accelerator: string;
  useWhitespaceSeparator?: boolean;
}): string {
  switch (options.platform) {
    case 'darwin':
      return options.accelerator
        .replaceAll('+', options.useWhitespaceSeparator ? ' ' : '')
        .replace('Command', '⌘')
        .replace('Control', '⌃')
        .replace('Option', '⌥')
        .replace('Shift', '⇧');
    default:
      return options.useWhitespaceSeparator
        ? options.accelerator.replaceAll('+', ' + ')
        : options.accelerator;
  }
}
