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

const uuid = {
  3: /^[0-9A-F]{8}-[0-9A-F]{4}-3[0-9A-F]{3}-[0-9A-F]{4}-[0-9A-F]{12}$/i,
  4: /^[0-9A-F]{8}-[0-9A-F]{4}-4[0-9A-F]{3}-[89AB][0-9A-F]{3}-[0-9A-F]{12}$/i,
  5: /^[0-9A-F]{8}-[0-9A-F]{4}-5[0-9A-F]{3}-[89AB][0-9A-F]{3}-[0-9A-F]{12}$/i,
  all: /^[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}$/i
};

const PORT_REGEX = /:\d+$/;

export function parseIp(addr){  
  addr = addr || '';
  return addr.replace(PORT_REGEX, '');
}

export function isMatch(obj, searchValue, {searchableProps, cb}) {
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

export function isUUID(str, version = 'all') {  
  const pattern = uuid[version];
  return pattern && pattern.test(str);
}