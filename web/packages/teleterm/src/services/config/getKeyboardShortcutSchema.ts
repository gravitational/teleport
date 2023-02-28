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

import { z } from 'zod';

import { Platform } from 'teleterm/mainProcess/types';

const VALID_SHORTCUT_MESSAGE =
  'A valid shortcut contains zero or more modifiers (like "Shift") and a single key code (like "A" or "Tab"), combined by the "+" character.';

export function invalidKeyCodeIssue(wrongKeyCode: string): z.IssueData {
  return {
    code: z.ZodIssueCode.custom,
    message: `"${wrongKeyCode}" cannot be used as a key code. ${VALID_SHORTCUT_MESSAGE}`,
  };
}

export function invalidModifierIssue(wrongModifier: string): z.IssueData {
  return {
    code: z.ZodIssueCode.custom,
    message: `"${wrongModifier}" cannot be used as a modifier. ${VALID_SHORTCUT_MESSAGE}`,
  };
}

export function duplicateModifierIssue(): z.IssueData {
  return {
    code: z.ZodIssueCode.custom,
    message: `Duplicate modifier found. ${VALID_SHORTCUT_MESSAGE}`,
  };
}

export function getKeyboardShortcutSchema(platform: Platform) {
  const allowedModifiers = getSupportedModifiers(platform);

  return z
    .string()
    .transform(s => s.split('+'))
    .transform(putModifiersFirst(allowedModifiers))
    .superRefine(validateKeyCodeAndModifiers(allowedModifiers))
    .transform(s => s.join('+'));
}

function putModifiersFirst(
  allowedModifiers: string[]
): (tokens: string[]) => string[] {
  return tokens =>
    tokens.sort((a, b) => {
      if (allowedModifiers.indexOf(a) === -1) {
        return 1;
      }
      if (allowedModifiers.indexOf(b) === -1) {
        return -1;
      }
      return allowedModifiers.indexOf(a) - allowedModifiers.indexOf(b);
    });
}

function validateKeyCodeAndModifiers(
  allowedModifiers: string[]
): (tokens: string[], ctx: z.RefinementCtx) => void {
  return (tokens, ctx) => {
    // empty accelerator disables the shortcut
    if (tokens.join('') === '') {
      return z.NEVER;
    }

    const [expectedKeyCode, ...expectedModifiers] = [...tokens].reverse();

    if (!ALLOWED_KEY_CODES.includes(expectedKeyCode)) {
      ctx.addIssue(invalidKeyCodeIssue(expectedKeyCode));
      return z.NEVER;
    }

    if (expectedModifiers.length !== new Set(expectedModifiers).size) {
      ctx.addIssue(duplicateModifierIssue());
      return z.NEVER;
    }

    expectedModifiers.forEach(expectedModifier => {
      if (!allowedModifiers.includes(expectedModifier)) {
        ctx.addIssue(invalidModifierIssue(expectedModifier));
      }
    });
  };
}

/** Returns allowed modifiers for a given platform in the correct order. */
function getSupportedModifiers(platform: Platform): string[] {
  switch (platform) {
    case 'win32':
    case 'linux':
      return ['Ctrl', 'Alt', 'Shift'];
    case 'darwin':
      return ['Command', 'Control', 'Option', 'Shift'];
  }
}

function generateRange(start: number, end: number): string[] {
  return new Array(end - start + 1)
    .fill(undefined)
    .map((_, i) => (i + start).toString());
}

const ALLOWED_KEY_CODES = [
  ...generateRange(0, 9), // 0-9 range
  ...generateRange(1, 24).map(key => `F${key}`), // F1-F24 keys
  'A',
  'B',
  'C',
  'D',
  'E',
  'F',
  'G',
  'H',
  'I',
  'J',
  'K',
  'L',
  'M',
  'N',
  'O',
  'P',
  'Q',
  'R',
  'S',
  'T',
  'U',
  'V',
  'W',
  'X',
  'Y',
  'Z',
  'Space',
  'Tab',
  'CapsLock',
  'NumLock',
  'ScrollLock',
  'Backspace',
  'Delete',
  'Insert',
  'Enter',
  'Up',
  'Down',
  'Left',
  'Right',
  'Home',
  'End',
  'PageUp',
  'PageDown',
  'Escape',
  ',',
  '.',
  '/',
  '\\',
  '`',
  '-',
  '=',
  ';',
  "'",
  '[',
  ']',
];
