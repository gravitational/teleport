/*
Copyright 2021-2022 Gravitational, Inc.

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
