/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
 * `collectKeys` gathers object keys recursively and returns them. Arrays are
 * traversed, but are transparent. Returns null if the value is not an object
 * or array of objects, and an empty array of no keys are present.
 *
 * @param value Value from which keys will be collected
 * @param prefix An optional value to be prepended to all keys returned
 * @returns An array of the keys collected (if any) or null
 */
export const collectKeys = (value: unknown, prefix: string = '') => {
  if (typeof value !== 'object' || value === null) {
    return prefix ? [prefix] : null;
  }

  if (Array.isArray(value)) {
    return value.flatMap(val => {
      return collectKeys(val, prefix);
    });
  }

  return Object.entries(value).flatMap(([k, v]) => {
    return collectKeys(v, `${prefix}.${k}`);
  });
};
