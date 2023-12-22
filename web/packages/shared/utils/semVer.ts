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

export const compareSemVers = (a: string, b: string): -1 | 1 | 0 => {
  const splitA = a.split('.');
  const splitB = b.split('.');

  if (splitA.length < 3 || splitB.length < 3) {
    return -1;
  }

  const majorA = parseInt(splitA[0]);
  const majorB = parseInt(splitB[0]);
  if (majorA !== majorB) {
    return majorA > majorB ? 1 : -1;
  }

  const minorA = parseInt(splitA[1]);
  const minorB = parseInt(splitB[1]);
  if (minorA !== minorB) {
    return minorA > minorB ? 1 : -1;
  }

  const patchA = parseInt(splitA[2].split('-')[0]);
  const patchB = parseInt(splitB[2].split('-')[0]);
  if (patchA !== patchB) {
    return patchA > patchB ? 1 : -1;
  }

  return 0;
};
