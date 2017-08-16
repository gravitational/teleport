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

import localStorage, { KeysEnum } from 'app/services/localStorage';
import { BearerToken } from 'app/services/session';
import { expect, spyOn } from 'app/__tests__/';
import * as apiData from 'app/__tests__/apiData';

describe('services/localStorage', () => {
  
  afterEach(() => {
    expect.restoreSpies();
  })

  describe('bearer token', () => {
    const bearerToken = new BearerToken(apiData.bearerToken);
    const serializedToken = JSON.stringify(bearerToken);

    it('should put and retrieve bearer token from browser localStorage', () => {
      spyOn(window.localStorage, 'setItem'); 
      spyOn(window.localStorage, 'getItem').andReturn(serializedToken);      
      
      localStorage.setBearerToken(bearerToken);      
      const actual = localStorage.getBearerToken();

      expect(window.localStorage.setItem).toHaveBeenCalledWith(KeysEnum.TOKEN, serializedToken);
      expect(actual.accessToken).toEqual(bearerToken.accessToken);      
    });
  });    
})