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

import cfg from 'teleport/config';
import auth from 'teleport/services/auth';
import api from 'teleport/services/api';

describe('services/auth', () => {
  beforeEach(() => {
    global.u2f = {
      sign() {},
      register() {},
    };
  });

  afterEach(() => {
    jest.clearAllMocks();
    delete global.u2f;
  });

  // sample data
  const user = 'user@example.com';
  const password = 'sample_pass';
  const inviteToken = 'invite_token';
  const secretToken = 'sample_secret_token';

  describe('login()', () => {
    const user = 'user@example.com';
    const email = user;
    const submitData = {
      user: email,
      pass: password,
      second_factor_token: undefined,
    };

    it('should login with email', async () => {
      jest.spyOn(api, 'post').mockImplementation(() => Promise.resolve());
      expect.assertions(1);
      await auth.login(email, password).then(() => {
        expect(api.post).toHaveBeenCalledWith(
          cfg.api.sessionPath,
          submitData,
          false
        );
      });
    });

    it('should login with OTP', async () => {
      jest.spyOn(api, 'post').mockImplementation(() => Promise.resolve());
      const data = {
        ...submitData,
        second_factor_token: 'xxx',
      };

      expect.assertions(1);
      await auth.login(email, password, 'xxx').then(() => {
        expect(api.post).toHaveBeenCalledWith(cfg.api.sessionPath, data, false);
      });
    });
  });

  describe('loginWithU2f()', () => {
    it('should login', () => {
      const dummyResponse = { appId: 'xxx' };
      jest
        .spyOn(api, 'post')
        .mockImplementation(() => Promise.resolve(dummyResponse));
      jest.spyOn(global.u2f, 'sign').mockImplementation((a, b, c, d) => {
        d(dummyResponse);
      });

      expect.assertions(1);
      return auth.loginWithU2f(user, password).then(() => {
        expect(window.u2f.sign).toHaveBeenCalled();
      });
    });

    it('should handle error', () => {
      const dummyResponse = { appId: 'xxx' };
      jest
        .spyOn(api, 'post')
        .mockImplementation(() => Promise.resolve(dummyResponse));
      jest.spyOn(window.u2f, 'sign').mockImplementation((a, b, c, d) => {
        d({ errorCode: '404' });
      });

      expect.assertions(1);
      auth.loginWithU2f(user, password).catch(() => {
        expect(window.u2f.sign).toHaveBeenCalled();
      });
    });
  });

  describe('registerWith2FA()', () => {
    it('should accept invite with 2FA', () => {
      const submitData = {
        invite_token: 'invite_token',
        pass: 'c2FtcGxlX3Bhc3M=',
        token: 'invite_token',
        u2f_register_response: undefined,
      };

      jest.spyOn(api, 'post').mockImplementation(() => Promise.resolve());

      expect.assertions(1);
      return auth
        .registerWith2FA(password, secretToken, inviteToken)
        .then(() => {
          expect(api.post).toHaveBeenCalledWith(
            cfg.api.userTokenInviteDonePath,
            submitData,
            false
          );
        });
    });
  });

  describe('registerWithU2F()', () => {
    xit('should accept invite with U2F', () => {
      const appId = 'xxx';
      const dummyResponse = { appId };

      jest.spyOn(api, 'post').mockImplementation(() => Promise.resolve());
      jest
        .spyOn(api, 'get')
        .mockImplementation(() => Promise.resolve(dummyResponse));
      jest.spyOn(window.u2f, 'register').mockImplementation((a, b, c, d) => {
        d(dummyResponse);
      });

      expect.assertions(2);
      return auth.registerWithU2F(password, inviteToken).then(() => {
        expect(api.get).toHaveBeenCalledWith(
          `/v1/webapi/u2f/signuptokens/${inviteToken}`
        );
        expect(api.post).toHaveBeenCalledWith(
          '/v1/webapi/u2f/users',
          {
            invite_token: 'invite_token',
            pass: 'c2FtcGxlX3Bhc3M=',
            token: 'invite_token',

            u2f_register_response: {
              appId: appId,
            },
          },
          false
        );
      });
    });

    it('should handle error', () => {
      const dummyResponse = { appId: 'xxx' };
      jest.spyOn(api, 'post').mockImplementation(() => Promise.resolve());
      jest
        .spyOn(api, 'get')
        .mockImplementation(() => Promise.resolve(dummyResponse));
      jest.spyOn(window.u2f, 'register').mockImplementation((a, b, c, d) => {
        d({ errorCode: '404' });
      });

      expect.assertions(1);
      return auth.registerWithU2F(password, inviteToken).catch(() => {
        expect(api.post).toHaveBeenCalledTimes(0);
      });
    });
  });
});
