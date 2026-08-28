/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
 * Parses a string ie: foo,bar,"quoted value"` into strings
 * array: ["foo", "bar", "quoted value"].
 *
 * JS version of backends `ParseSearchKeywords`:
 * https://github.com/gravitational/teleport/blob/06b15b28fb1fcccb5ccc1b79b355d1d6f26829bf/lib/client/api.go#L4621
 */
export function parseQuotedWordsDelimitedByComma(str: string): string[] {
  const delimiter = `,`;
  const tokens = [];
  let openQuotes = false;
  let tokenStart = 0;

  const strLen = str.length;

  for (let i = 0; i < str.length; i++) {
    let endOfToken = false;
    const ch = str.charAt(i);

    if (i + 1 == strLen) {
      endOfToken = true;
      i += 1;
    }

    switch (ch) {
      case '"':
        openQuotes = !openQuotes;
        break;
      case delimiter:
        if (!openQuotes) {
          endOfToken = true;
        }
    }
    if (endOfToken && i > tokenStart) {
      // Replaces all beginning and trailing spaces and quotes.
      const regex = /^[" ]+|[" ]+$/g;
      tokens.push(str.substring(tokenStart, i).replace(regex, ''));
      tokenStart = i + 1;
    }
  }

  return tokens;
}
