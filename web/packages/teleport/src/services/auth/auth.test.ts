/*
Copyright 2015-2022 Gravitational, Inc.

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

/* eslint-disable jest/no-conditional-expect */

describe('services/auth', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  // sample data
  const password = 'sample_pass';
  const email = 'user@example.com';

  test('login()', async () => {
    jest.spyOn(api, 'post').mockResolvedValue({});

    await auth.login(email, password, '');
    expect(api.post).toHaveBeenCalledWith(cfg.api.webSessionPath, {
      user: email,
      pass: password,
      second_factor_token: '',
    });
  });

  test('login() OTP', async () => {
    jest.spyOn(api, 'post').mockResolvedValue({});
    const data = {
      user: email,
      pass: password,
      second_factor_token: 'xxx',
    };

    await auth.login(email, password, 'xxx');
    expect(api.post).toHaveBeenCalledWith(cfg.api.webSessionPath, data);
  });

  test('resetPassword()', async () => {
    jest.spyOn(api, 'put').mockResolvedValue({});
    const submitData = {
      token: 'tokenId',
      second_factor_token: '2fa_token',
      password: 'c2FtcGxlX3Bhc3M=',
    };

    await auth.resetPassword({
      req: {
        tokenId: 'tokenId',
        password: password,
        otpCode: '2fa_token',
      },
    });
    expect(api.put).toHaveBeenCalledWith(
      cfg.getPasswordTokenUrl(),
      expect.objectContaining(submitData)
    );
  });
});
