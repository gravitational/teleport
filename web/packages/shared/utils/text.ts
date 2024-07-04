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
 * pluralize adds an 's' to the given word if num is bigger than 1.
 */
// If you ever need to pluralize a word which cannot be pluralized by appending 's', just add a
// third optional argument which is the pluralized noun.
// https://api.rubyonrails.org/v7.0.4.2/classes/ActionView/Helpers/TextHelper.html#method-i-pluralize
export function pluralize(num: number, word: string) {
  if (num > 1) {
    return `${word}s`;
  }

  return word;
}

/**
 * capitalizeFirstLetter uppercases the first letter in the string.
 */
export function capitalizeFirstLetter(str: string) {
  if (!str) {
    return '';
  }
  return str[0].toUpperCase() + str.slice(1);
}
