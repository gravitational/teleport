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
import history from 'app/services/history';
import {createMemoryHistory} from 'react-router';
import {reactor, expect, api, Dfd, spyOn} from './../';
import actions from 'app/flux/user/actions';
import getters from 'app/flux/user/getters';
import {AuthProviderTypeEnum, AuthProviderEnum} from 'app/services/enums';
import {RECEIVE_USER} from 'app/flux/user/actionTypes';
import * as apiData from 'app/__tests__/apiData';

describe('flux/user/getters', () => {
  it('should return "user"', () => {
    const userName = apiData.userContext.userName;    
    reactor.dispatch(RECEIVE_USER, { name: userName });    
    expect(reactor.evaluate(getters.userName)).toEqual(userName);
  });
});

describe('flux/user/actions', () => {
  beforeEach(() => {
    spyOn(history, 'push');
  });

  afterEach(() => {
    reactor.reset();
    expect.restoreSpies();
  })

  history.init(createMemoryHistory())

  // sample data
  const inviteToken = 'd82e9f81b3826801af8da16cde3335cbffcef5f7e9490e880b3fcc3f894efcfb';
  const secretToken = 'sample_secret_token';
  const user = 'user@example.com';
  const email = user;
  const password = 'sample_pass';
  const err = { responseJSON: { message: 'error' } }

  describe('fetchInvite', () => {
    const inviteInfoSample = {
      invite_token: inviteToken,
      qr: "iVBORw0KG",
      user: "dada"
    };

    it('should handle loading state', () => {
      spyOn(api, 'get').andReturn(Dfd());
      actions.fetchInvite(inviteToken)
      expect(reactor.evaluate(getters.fetchingInvite)).toEqual({isProcessing: true});
    });

    it('should handle failed state', () => {
      const message = 'error';
      spyOn(api, 'get').andReturn(Dfd().reject(err));
      actions.fetchInvite(inviteToken)
      expect(reactor.evaluate(getters.fetchingInvite)).toEqual({isFailed: true, message});
    });

    it('should handle success state', () => {
      spyOn(api, 'get').andReturn(Dfd().resolve(inviteInfoSample));
      actions.fetchInvite(inviteToken)
      expect(reactor.evaluateToJS(getters.invite)).toEqual(inviteInfoSample);
      expect(reactor.evaluate(getters.fetchingInvite)).toEqual({isSuccess: true});
    });
  });

  describe('login()', () => {
    const oidcSsoProvider = { name: AuthProviderEnum.MS, type: AuthProviderTypeEnum.OIDC };
    const samlSsoProvider = { name: AuthProviderEnum.MS, type: AuthProviderTypeEnum.SAML };

    it('should login with email', () => {
      spyOn(auth, 'login').andReturn(Dfd().resolve(apiData.bearerToken));
      actions.login(email, password);
      expect(history.push).toHaveBeenCalledWith(cfg.routes.app, true);
    });

    it('should login with OIDC', () => {
      const expectedUrl = `localhost/v1/webapi/oidc/login/web?redirect_url=localhost%2Fweb&connector_id=${samlSsoProvider.name}`;
      actions.loginWithSso(oidcSsoProvider.name, oidcSsoProvider.type);
      expect(history.push).toHaveBeenCalledWith(expectedUrl, true);
    });

    it('should login with SAML', () => {
      const expectedUrl = `localhost/v1/webapi/saml/sso?redirect_url=localhost%2Fweb&connector_id=${samlSsoProvider.name}`;
      actions.loginWithSso(samlSsoProvider.name, samlSsoProvider.type);
      expect(history.push).toHaveBeenCalledWith(expectedUrl, true);
    });

    it('should login with U2F', () => {
      const dummyResponse = { appId: 'xxx' }
      spyOn(api, 'post').andReturn(Dfd().resolve(dummyResponse));
      spyOn(window.u2f, 'sign').andCall((a, b, c, d) => {
        d(dummyResponse)
      });

      actions.loginWithU2f(email, password);
      expect(window.u2f.sign).toHaveBeenCalled();
      expect(history.push).toHaveBeenCalledWith(cfg.routes.app, true);
    });

    it('should handle loginAttemp states', () => {
      spyOn(auth, 'login').andReturn(Dfd());
      actions.login(email, password);

      // processing
      let attemp = reactor.evaluateToJS(getters.loginAttemp);
      expect(attemp.isProcessing).toBe(true);

      // reject
      reactor.reset();
      spyOn(auth, 'login').andReturn(Dfd().reject(err));
      actions.login(email, password);
      attemp = reactor.evaluateToJS(getters.loginAttemp);
      expect(attemp.isFailed).toBe(true);
    });
  })

  it('acceptInvite() should accept invite with 2FA', () => {
    const submitData = {
      user,
      pass: password,
      second_factor_token: secretToken,
      invite_token: inviteToken
    }

    spyOn(api, 'post').andReturn(Dfd().resolve());
    actions.acceptInvite(user, password, secretToken, inviteToken);
    expect(api.post).toHaveBeenCalledWith(cfg.api.createUserPath, submitData, false);
    expect(history.push).toHaveBeenCalledWith(cfg.routes.app, true);
  });

  it('acceptInviteWithU2f() should accept invite with U2F', () => {
    const appId = 'xxx';
    const dummyResponse = { appId };
    let wasCalled = false;
    spyOn(api, 'post').andReturn(Dfd().resolve());
    spyOn(api, 'get').andReturn(Dfd().resolve(dummyResponse));
    spyOn(window.u2f, 'register').andCall((a, b, c, d) => {
      d(dummyResponse)
    });

    actions.acceptInviteWithU2f(user, password, inviteToken).done(() => wasCalled = true );
    expect(wasCalled).toBe(true);    
    expect(history.push).toHaveBeenCalledWith(cfg.routes.app, true);
  });

});
