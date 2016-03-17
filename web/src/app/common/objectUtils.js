/*
Copyright 2015 Gravitational, Inc.

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

module.exports.isMatch = function(obj, searchValue, {searchableProps, cb}) {
  searchValue = searchValue.toLocaleUpperCase();
  let propNames = searchableProps || Object.getOwnPropertyNames(obj);
  for (let i = 0; i < propNames.length; i++) {
    let targetValue = obj[propNames[i]];
    if (targetValue) {
      if(typeof cb === 'function'){
        let result = cb(targetValue, searchValue, propNames[i]);
        if(result === true){
          return result;
        }
      }

      if (targetValue.toString().toLocaleUpperCase().indexOf(searchValue) !== -1) {
        return true;
      }
    }
  }

  return false;
}
