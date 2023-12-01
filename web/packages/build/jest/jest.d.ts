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

// Ignore no-empty-interface in order to explicitly follow the types from Jest docs.
// Otherwise ESLint would autofix them.
/* eslint-disable @typescript-eslint/no-empty-interface */

// https://jestjs.io/docs/27.x/expect#expectextendmatchers
// https://redd.one/blog/practical-guide-to-custom-jest-matchers
interface CustomMatchers<R = unknown> {
  toEventuallyBeTrue(args: { waitFor: number; tick: number }): Promise<R>;
}

declare global {
  namespace jest {
    interface Expect extends CustomMatchers {}
    interface Matchers<R> extends CustomMatchers<R> {}
    interface InverseAssymetricMatchers extends CustomMatchers {}
  }
}
export {};
