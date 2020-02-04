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
    // setup u2f mocks
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
  const dummyU2fRegResponse = Promise.resolve({ appId: 'xxx' });
  const password = 'sample_pass';
  const email = 'user@example.com';

  test('login()', async () => {
    jest.spyOn(api, 'post').mockImplementation(() => Promise.resolve());

    await auth.login(email, password);
    expect(api.post).toHaveBeenCalledWith(
      cfg.api.sessionPath,
      {
        user: email,
        pass: password,
        second_factor_token: undefined,
      },
      false
    );
  });

  test('login() OTP', async () => {
    jest.spyOn(api, 'post').mockImplementation(() => Promise.resolve());
    const data = {
      user: email,
      pass: password,
      second_factor_token: 'xxx',
    };

    await auth.login(email, password, 'xxx');
    expect(api.post).toHaveBeenCalledWith(cfg.api.sessionPath, data, false);
  });

  test('loginWithU2f()', async () => {
    jest.spyOn(api, 'post').mockImplementation(() => dummyU2fRegResponse);
    jest.spyOn(global.u2f, 'sign').mockImplementation((a, b, c, d) => {
      d(dummyU2fRegResponse);
    });

    await auth.loginWithU2f(email, password);
    expect(window.u2f.sign).toHaveBeenCalled();
  });

  test('loginWithU2f() error', async () => {
    jest.spyOn(api, 'post').mockImplementation(() => dummyU2fRegResponse);
    jest.spyOn(window.u2f, 'sign').mockImplementation((a, b, c, d) => {
      d({ errorCode: '404' });
    });

    try {
      await auth.loginWithU2f(email, password);
    } catch (err) {
      expect(window.u2f.sign).toHaveBeenCalled();
    }
    expect.assertions(1);
  });

  test('resetPassword()', async () => {
    jest.spyOn(api, 'put').mockImplementation(() => Promise.resolve());
    const submitData = {
      token: 'tokenId',
      second_factor_token: '2fa_token',
      password: 'c2FtcGxlX3Bhc3M=',
      u2f_register_response: undefined,
    };

    await auth.resetPassword('tokenId', password, '2fa_token');
    expect(api.put).toHaveBeenCalledWith(
      cfg.getPasswordTokenUrl(),
      submitData,
      false
    );
  });

  test('resetPasswordU2F()', async () => {
    jest.spyOn(api, 'post').mockImplementation(() => dummyU2fRegResponse);
    jest.spyOn(api, 'get').mockImplementation(() => Promise.resolve({}));
    jest.spyOn(window.u2f, 'register').mockImplementation((a, b, c, d) => {
      d(dummyU2fRegResponse);
    });

    const submitted = {
      second_factor_token: null,
      password: 'c2FtcGxlX3Bhc3M=',
      token: 'tokenId',
      u2f_register_response: {
        appId: 'xxx',
      },
    };

    await auth.resetPasswordWithU2f('tokenId', password);
    expect(api.get).toHaveBeenCalledWith(
      cfg.getU2fCreateUserChallengeUrl('tokenId')
    );
    expect(api.put).toHaveBeenCalledWith(
      cfg.getPasswordTokenUrl(),
      submitted,
      false
    );
  });

  test('resetPasswordU2F() error', async () => {
    jest.spyOn(api, 'post').mockImplementation(() => dummyU2fRegResponse);
    jest.spyOn(api, 'get').mockImplementation(() => dummyU2fRegResponse);
    jest.spyOn(window.u2f, 'register').mockImplementation((a, b, c, d) => {
      d({ errorCode: '404' });
    });

    try {
      await auth.resetPasswordWithU2f('tokenId', password);
    } catch (err) {
      expect(api.post).toHaveBeenCalledTimes(0);
    }

    expect.assertions(1);
  });
});
