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
 * `formatDuration` formats a given duration (in seconds) as a duration string
 * supporting h, m and s units with an optional separator (e.g., "1h 2m 3s").
 *
 * @param duration object containting number of seconds to format
 * @param options optional config to provide a separator
 * @returns a duration string
 */
export function formatDuration(
  duration:
    | {
        seconds: number;
      }
    | undefined
    | null,
  options?: {
    separator?: string;
  }
): string {
  const { seconds = 0 } = duration ?? {};
  const { separator = '' } = options ?? {};

  if (!seconds) return '0s';

  return units
    .reduce(
      ({ remainder, fmt }, { unit, denominator }) => {
        const value = Math.floor(remainder / denominator);
        return {
          remainder: remainder % denominator,
          fmt: value > 0 ? [...fmt, `${value}${unit}`] : fmt,
        };
      },
      { remainder: seconds, fmt: [] }
    )
    .fmt.join(separator);
}

// Order is important - largest denominators first
const units = [
  { unit: 'h', denominator: 3_600 },
  { unit: 'm', denominator: 60 },
  { unit: 's', denominator: 1 },
];
