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

export function formatDuration(
  duration:
    | {
        seconds: number;
        nanoseconds?: number;
      }
    | undefined
    | null,
  options?: {
    separator?: string;
  }
): string {
  const { seconds = 0, nanoseconds = 0 } = duration ?? {};
  const { separator = '' } = options ?? {};
  // Convert everything to nanoseconds for easier calculation
  const totalNanoseconds = seconds * 1_000_000_000 + nanoseconds;

  if (totalNanoseconds === 0) {
    return '0s';
  }

  // Calculate each component
  const components = {
    hours: 0,
    minutes: 0,
    seconds: 0,
    milliseconds: 0,
    microseconds: 0,
    nanoseconds: 0,
  };

  let remaining = totalNanoseconds;

  // Hours (3,600,000,000,000 nanoseconds)
  components.hours = Math.floor(remaining / 3_600_000_000_000);
  remaining %= 3_600_000_000_000;

  // Minutes (60,000,000,000 nanoseconds)
  components.minutes = Math.floor(remaining / 60_000_000_000);
  remaining %= 60_000_000_000;

  // Seconds (1,000,000,000 nanoseconds)
  components.seconds = Math.floor(remaining / 1_000_000_000);
  remaining %= 1_000_000_000;

  // Milliseconds (1,000,000 nanoseconds)
  components.milliseconds = Math.floor(remaining / 1_000_000);
  remaining %= 1_000_000;

  // Microseconds (1,000 nanoseconds)
  components.microseconds = Math.floor(remaining / 1_000);
  remaining %= 1_000;

  // Nanoseconds
  components.nanoseconds = remaining;

  // Build the result string
  const parts: string[] = [];

  if (components.hours) parts.push(`${components.hours}h`);
  if (components.minutes) parts.push(`${components.minutes}m`);
  if (components.seconds) parts.push(`${components.seconds}s`);
  if (components.milliseconds) parts.push(`${components.milliseconds}ms`);
  if (components.microseconds) parts.push(`${components.microseconds}µs`);
  if (components.nanoseconds) parts.push(`${components.nanoseconds}ns`);

  return parts.join(separator);
}
