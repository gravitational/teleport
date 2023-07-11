/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { Attempt } from 'shared/hooks/useAttemptNext';

import { Auth2faType, PrimaryAuthType } from 'shared/services';

import { NewFlow, StepComponentProps } from 'design/StepSlider';

import { RecoveryCodes, ResetToken } from 'teleport/services/auth';

export type UseTokenState = {
  auth2faType: Auth2faType;
  primaryAuthType: PrimaryAuthType;
  isPasswordlessEnabled: boolean;
  fetchAttempt: Attempt;
  submitAttempt: Attempt;
  clearSubmitAttempt: () => void;
  onSubmit: (password: string, otpCode?: string, deviceName?: string) => void;
  onSubmitWithWebauthn: (password?: string, deviceName?: string) => void;
  resetToken: ResetToken;
  recoveryCodes: RecoveryCodes;
  redirect: () => void;
  success: boolean;
  finishedRegister: () => void;
  privateKeyPolicyEnabled: boolean;
};

export type NewCredentialsProps = UseTokenState & {
  resetMode?: boolean;
  isDashboard: boolean;
};

export type RegisterSuccessProps = {
  redirect(): void;
  resetMode: boolean;
  username?: string;
  isDashboard: boolean;
};

export type LoginFlow = Extract<PrimaryAuthType, 'passwordless' | 'local'>;
export type SliderProps = StepComponentProps & {
  changeFlow(f: NewFlow<LoginFlow>): void;
};
