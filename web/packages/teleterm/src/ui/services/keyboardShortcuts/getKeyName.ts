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

/**
 * Based on tabby https://github.com/Eugeny/tabby/blob/7a8108b20d9cbaab636c932ceaf4bacc710d6a40/tabby-core/src/services/hotkeys.util.ts
 *
 * The MIT License (MIT)
 *
 * Copyright (c) 2017 Eugene Pankov
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 */

const REGEX_LATIN_KEYNAME = /^[A-Za-z]$/;

/**
 * Returns a key name in a way that makes some keys independent of their physical placement on the US QWERTY layout.
 *
 * First, we check if the printed character is from the range A-Z (case-insensitive).
 * This check bounds the letters to the (changeable) keyboard layout, not the physical keys.
 * For example, in the Dvorak layout, the "K" and "T" keys are interchanged (compared to US QWERTY).
 * By relying on the printed character, we are independent of the layout.
 * This regex also excludes non-Latin characters, which should be handled by physical codes,
 * because `KeyboardEvent.key` will be a letter from that alphabet.
 * Most of these keyboards follow the standard US QWERTY layout,
 * so it is possible for `KeyboardEvent.code` to work as a fallback.
 *
 * The rest of the keys are handled by their physical code.
 * It is common in many layouts that the user has to press a modifier to input a character
 * which is available on US QWERTY without any modifiers.
 * For example, in Czech QWERTY there is no "1" key (it is on upper row) -
 * the user has to press "Shift+(plus key)" to get "1".
 * The resulting character is still "1" as in US QWERTY,
 * but because "Shift" was pressed we would interpret it as a different shortcut ("Shift+1", not "1").
 *
 * The above mechanism is not perfect, because only A-Z keys are mapped to the active layout.
 * Keys like comma are still tied to the physical keys in the US QWERTY.
 * */
export function getKeyName(event: KeyboardEvent): string {
  if (REGEX_LATIN_KEYNAME.test(event.key)) {
    // Handle Dvorak etc. via the reported "character" instead of the scancode
    return event.key.toUpperCase();
  }
  let key = event.code;
  key = key.replace('Key', '');
  key = key.replace('Arrow', '');
  key = key.replace('Digit', '');
  key = PHYSICAL_CODE_TO_PRINTABLE_CHARACTER[key] ?? key;
  return key;
}

const PHYSICAL_CODE_TO_PRINTABLE_CHARACTER = {
  Comma: ',',
  Period: '.',
  Slash: '/',
  Backslash: '\\',
  Backquote: '`',
  Minus: '-',
  Equal: '=',
  Semicolon: ';',
  Quote: "'",
  BracketLeft: '[',
  BracketRight: ']',
};
