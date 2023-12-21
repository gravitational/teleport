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
 * match searches for a match, given a search value, through an array of objects.
 *
 * @param obj An array of objects to search for matches
 * @param searchValue The value to look for in obj
 * @param searchableProps The properties in obj that we can match searchValue
 * @param cb Callback function to handle special cases like data format differences.
 *    E.g: user sees date as '2020/01/15', but data is stored as 'Wed Jan 15 2020',
 *         we must first convert data as how user sees it, then apply match.
 *
 *    cb(target: any[], searchValue: string, prop: string):
 *      - param target: to apply the searchValue against (find a match)
 *      - param searchValue: the value to look for in target
 *      - param prop: the current obj property name where searchValue may be matched
 */
export default function match<T>(
  obj: T,
  searchValue: string,
  {
    searchableProps,
    cb,
  }: {
    searchableProps: (keyof T & string)[];
    cb?: MatchCallback<T>;
  }
) {
  searchValue = searchValue.toLocaleUpperCase();

  let propNames =
    searchableProps ||
    (Object.getOwnPropertyNames(obj) as (keyof T & string)[]);
  for (let i = 0; i < propNames.length; i++) {
    let targetValue = obj[propNames[i]];
    if (targetValue) {
      if (typeof cb === 'function') {
        let result = cb(targetValue, searchValue, propNames[i]);
        if (result === true) {
          return result;
        }
      }

      if (
        targetValue.toString().toLocaleUpperCase().indexOf(searchValue) !== -1
      ) {
        return true;
      }
    }
  }

  return false;
}

export type MatchCallback<T> = (
  targetValue: any,
  searchValue: string,
  propName: keyof T & string
) => boolean;
