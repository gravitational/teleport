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

import cfg from 'app/config';
import auth from 'app/services/auth';
import {reactor, expect, api, Dfd, spyOn} from './';

describe('services/auth', () => {
  
  afterEach(() => {
    reactor.reset();
    expect.restoreSpies();
  })

  // sample data    
  const user = 'user@example.com';
  const password = 'sample_pass';
  const inviteToken = 'd82e9f81b3826801af8da16cde3335cbffcef5f7e9490e880b3fcc3f894efcfb';
  const secretToken = 'sample_secret_token';

  describe('login()', () => {        
    const user = 'user@example.com';
    const email = user;    
    const submitData = {
      user: email,
      pass: password,
      second_factor_token: undefined
    };

    it('should login with email', () => {      
      spyOn(api, 'post').andReturn(Dfd().resolve());            
      let wasCalled = false;
      auth.login(email, password).done(() => wasCalled = true);
      expect(api.post).toHaveBeenCalledWith(cfg.api.sessionPath, submitData, false);
      expect(wasCalled).toBe(true);
    });

    it('should login with OTP', () => {
      spyOn(api, 'post');      
      const data = {        
        ...submitData,
        second_factor_token: 'xxx'
      };

      auth.login(email, password, 'xxx')            
      expect(api.post).toHaveBeenCalledWith(cfg.api.sessionPath, data, false);
    });
  });
    
  describe('loginWithU2f()', () => {
    it('should login', () => {
      const dummyResponse = { appId: 'xxx' }
      spyOn(api, 'post').andReturn(Dfd().resolve(dummyResponse));
      spyOn(window.u2f, 'sign').andCall((a, b, c, d) => {
        d(dummyResponse)
      });

      auth.loginWithU2f(user, password);
      expect(window.u2f.sign).toHaveBeenCalled();      
    });    

    it('should handle error', () => {
      const dummyResponse = { appId: 'xxx' }
      let wasCalled = false;
      spyOn(api, 'post').andReturn(Dfd().resolve(dummyResponse));
      spyOn(window.u2f, 'sign').andCall((a, b, c, d) => {
        d({ errorCode: '404' })
      });

      auth.loginWithU2f(user, password).fail(() => wasCalled = true )
      expect(window.u2f.sign).toHaveBeenCalled();      
      expect(wasCalled).toBe(true);
    })
  })
      
  describe('acceptInvite()', () => {
    it('should accept invite with 2FA', () => {
      let wasCalled = false;
      const submitData = {
        user,
        pass: password,
        second_factor_token: secretToken,
        invite_token: inviteToken
      }
      
      spyOn(api, 'post').andReturn(Dfd().resolve());
      auth.acceptInvite(user, password, secretToken, inviteToken).done(() => wasCalled = true);    
      expect(api.post).toHaveBeenCalledWith(cfg.api.createUserPath, submitData, false);    
      expect(wasCalled).toBe(true);
    });
  })

  describe('acceptInviteWithU2f()', () => {
    it('should accept invite with U2F', () => {
      const appId = 'xxx';
      const dummyResponse = { appId };
      let wasCalled = false;
      spyOn(api, 'post').andReturn(Dfd().resolve());
      spyOn(api, 'get').andReturn(Dfd().resolve(dummyResponse));
      spyOn(window.u2f, 'register').andCall((a, b, c, d) => {
        d(dummyResponse)
      });

      auth.acceptInviteWithU2f(user, password, inviteToken).done(() => wasCalled = true);
      
      expect(wasCalled).toBe(true);
      expect(api.get).toHaveBeenCalledWith(`/v1/webapi/u2f/signuptokens/${inviteToken}`);
      expect(api.post).toHaveBeenCalledWith("/v1/webapi/u2f/users", {
        "user": user,
        "pass": password,
        "u2f_register_response": {
          "appId": appId
        },
        "invite_token": inviteToken
      }, false);    
    });

    it('should handle error', () => {
      const appId = 'xxx';
      const dummyResponse = { appId };
      let wasCalled = false;
      spyOn(api, 'post').andReturn(Dfd().resolve());
      spyOn(api, 'get').andReturn(Dfd().resolve(dummyResponse));
      spyOn(window.u2f, 'register').andCall((a, b, c, d) => {
        d({ errorCode: '404' })
      });

      auth.acceptInviteWithU2f(user, password, inviteToken).fail(() => wasCalled = true);      
      expect(wasCalled).toBe(true);
      expect(api.post).toNotHaveBeenCalled();      
    })

  });
});
