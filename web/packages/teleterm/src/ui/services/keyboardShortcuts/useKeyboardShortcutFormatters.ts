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

import { KeyboardShortcutAction } from '../../../services/config';
import { useAppContext } from '../../appContextProvider';
import { Platform } from '../../../mainProcess/types';

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
