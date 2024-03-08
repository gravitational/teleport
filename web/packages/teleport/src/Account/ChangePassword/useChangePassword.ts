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

import authService from 'teleport/services/auth';
import cfg from 'teleport/config';
import { MfaChallengeScope } from 'teleport/services/auth/auth';

export default function useChangePassword() {
  function changePassword(
    oldPassword: string,
    newPassword: string,
    secondFactorToken: string
  ) {
    return authService.changePassword({
      oldPassword,
      newPassword,
      secondFactorToken,
    });
  }

  async function changePasswordWithWebauthn(
    oldPassword: string,
    newPassword: string
  ) {
    const credential = await authService.fetchWebAuthnChallenge({
      scope: MfaChallengeScope.CHANGE_PASSWORD,
      userVerificationRequirement: 'discouraged',
    });
    return authService.changePassword({
      oldPassword,
      newPassword,
      secondFactorToken: '',
      credential,
    });
  }

  return {
    changePassword,
    changePasswordWithWebauthn,
    preferredMfaType: cfg.getPreferredMfaType(),
    auth2faType: cfg.getAuth2faType(),
  };
}

export type State = ReturnType<typeof useChangePassword>;
