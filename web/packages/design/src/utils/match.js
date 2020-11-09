/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
export default function match(obj, searchValue, { searchableProps, cb }) {
  searchValue = searchValue.toLocaleUpperCase();
  let propNames = searchableProps || Object.getOwnPropertyNames(obj);
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
        targetValue
          .toString()
          .toLocaleUpperCase()
          .indexOf(searchValue) !== -1
      ) {
        return true;
      }
    }
  }

  return false;
}
