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

export enum StatusEnum {
  LOADING = 'LOADING',
  PLAYING = 'PLAYING',
  PAUSED = 'PAUSED',
  COMPLETE = 'COMPLETE',
  ERROR = 'ERROR',
}

export function formatDisplayTime(ms: number) {
  if (ms <= 0) {
    return '00:00';
  }

  const totalSec = Math.floor(ms / 1000);
  const totalDays = (totalSec % 31536000) % 86400;
  const h = Math.floor(totalDays / 3600);
  const m = Math.floor((totalDays % 3600) / 60);
  const s = (totalDays % 3600) % 60;

  return `${h > 0 ? h + ':' : ''}${m.toString().padStart(2, '0')}:${s
    .toString()
    .padStart(2, '0')}`;
}
