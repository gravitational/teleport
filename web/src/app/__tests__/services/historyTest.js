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

import expect from 'expect';
import history from 'app/services/history';
import { createMemoryHistory } from 'react-router';
import cfg from 'app/config';

let spyOn = expect.spyOn;

history.init( new createMemoryHistory());

describe('services/history', function () {
  
  const fallbackRoute = cfg.routes.app;  
  const browserHistory = history.original(/* be default returns inMemory history*/);

  beforeEach(function () {    
    spyOn(browserHistory, 'push');
    spyOn(history, 'getRoutes');  
    spyOn(history, '_pageRefresh');          
  });

  afterEach(function () {
    expect.restoreSpies();
  });

  describe('canPush', function () {
        
    const tryAndExpectBrowserHistoryWith = (actual, expected) => {
      history.push(actual)
      expect(browserHistory.push).toHaveBeenCalledWith(expected);
    }
    
    it('should push if allowed else fallback to default route', function () {
      history.getRoutes.andReturn(['/valid', '/']);      
      tryAndExpectBrowserHistoryWith('invalid', fallbackRoute);
      tryAndExpectBrowserHistoryWith('.', fallbackRoute);
      tryAndExpectBrowserHistoryWith('/valid/test', fallbackRoute);
      tryAndExpectBrowserHistoryWith('@#4', fallbackRoute);
      tryAndExpectBrowserHistoryWith('/valid', '/valid');      
      tryAndExpectBrowserHistoryWith('', '');      
      tryAndExpectBrowserHistoryWith('/', '/');      
    })

    it('should refresh a page if called withRefresh=true', function () {
      let route = '/';
      history.getRoutes.andReturn([route]);            
      history.push(route, true)
      expect(history._pageRefresh).toHaveBeenCalledWith(route);
    })
  })

  describe('createRedirect()', function () {    
    it('should make valid redirect url', function () {
      let route = '/valid';
      let location = browserHistory.createLocation(route);
      history.getRoutes.andReturn([route]);      
      expect(history.createRedirect(location)).toEqual(cfg.baseUrl + route);            
    });    
  });    
})