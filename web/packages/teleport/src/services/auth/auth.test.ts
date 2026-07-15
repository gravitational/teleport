/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import cfg from 'teleport/config';
import api from 'teleport/services/api';
import auth from 'teleport/services/auth';

describe('services/auth', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  // sample data
  const password = 'sample_pass';
  const email = 'user@example.com';

  test('login()', async () => {
    jest.spyOn(api, 'postWithOptions').mockResolvedValue({});

    await auth.login(email, password, '');
    expect(api.postWithOptions).toHaveBeenCalledWith(
      cfg.api.webSessionPath,
      expect.objectContaining({
        data: {
          user: email,
          pass: password,
          second_factor_token: '',
        },
      })
    );
  });

  test('login() OTP', async () => {
    jest.spyOn(api, 'postWithOptions').mockResolvedValue({});
    const data = {
      user: email,
      pass: password,
      second_factor_token: 'xxx',
    };

    await auth.login(email, password, 'xxx');
    expect(api.postWithOptions).toHaveBeenCalledWith(
      cfg.api.webSessionPath,
      expect.objectContaining({ data })
    );
  });

  describe('getMfaChallengeResponseForAdminAction', () => {
    const original = {
      second_factor: cfg.auth.second_factor,
      second_factors: cfg.auth.second_factors,
    };
    afterEach(() => {
      cfg.auth.second_factor = original.second_factor;
      cfg.auth.second_factors = original.second_factors;
    });

    test('skips the challenge when admin MFA is not enforced', async () => {
      cfg.auth.second_factors = ['otp', 'webauthn'];
      const getMfaChallenge = jest.spyOn(auth, 'getMfaChallenge');

      const result = await auth.getMfaChallengeResponseForAdminAction(true);

      expect(result).toBeUndefined();
      expect(getMfaChallenge).not.toHaveBeenCalled();
    });

    test('falls back to server-side check when enforcement is unknown', async () => {
      // Older proxy/auth: second_factors empty and legacy field collapses to off.
      cfg.auth.second_factors = [];
      cfg.auth.second_factor = 'off';
      const getMfaChallenge = jest
        .spyOn(auth, 'getMfaChallenge')
        .mockResolvedValue(undefined);

      await auth.getMfaChallengeResponseForAdminAction(true);

      expect(getMfaChallenge).toHaveBeenCalled();
    });

    test('SSO-only cluster fetches the challenge with allowReuse', async () => {
      cfg.auth.second_factors = ['sso'];
      cfg.auth.second_factor = 'off';
      const getMfaChallenge = jest
        .spyOn(auth, 'getMfaChallenge')
        .mockResolvedValue(undefined);

      await auth.getMfaChallengeResponseForAdminAction(true);

      expect(getMfaChallenge).toHaveBeenCalledWith(
        expect.objectContaining({ allowReuse: true })
      );
    });
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
