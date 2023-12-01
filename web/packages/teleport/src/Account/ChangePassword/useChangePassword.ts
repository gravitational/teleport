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

export default function useChangePassword() {
  function changePassword(oldPass: string, newPass: string, otpToken: string) {
    return authService.changePassword(oldPass, newPass, otpToken);
  }

  function changePasswordWithWebauthn(oldPass: string, newPass: string) {
    return authService.changePasswordWithWebauthn(oldPass, newPass);
  }
  return {
    changePassword,
    changePasswordWithWebauthn,
    preferredMfaType: cfg.getPreferredMfaType(),
    auth2faType: cfg.getAuth2faType(),
  };
}

export type State = ReturnType<typeof useChangePassword>;
