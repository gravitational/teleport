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
 * pipe takes an array of unary functions as an argument. It returns a function that accepts a value
 * that's going to be passed through the supplied functions.
 *
 * @example
 * // Without pipe.
 * const add1ThenDouble = (x) => double(add1(x));
 *
 * // With pipe.
 * const add1ThenDouble = pipe(add1, double);
 */
export const pipe =
  <PipeSubject>(...fns: Array<(pipeSubject: PipeSubject) => PipeSubject>) =>
  (x: PipeSubject) =>
    fns.reduce((v, f) => f(v), x);
