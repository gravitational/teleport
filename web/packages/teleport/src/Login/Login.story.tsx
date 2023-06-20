/*
Copyright 2019-2022 Gravitational, Inc.

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

import React from 'react';

import LoginSuccess from './LoginSuccess';
import { LoginFailed } from './LoginFailed';
import { Login } from './Login';
import { State } from './useLogin';

export default {
  title: 'Teleport/Login',
};

export const MfaOff = () => <Login {...sample} />;
export const Otp = () => <Login {...sample} auth2faType="otp" />;
export const Webauthn = () => <Login {...sample} auth2faType="webauthn" />;
export const Optional = () => <Login {...sample} auth2faType="optional" />;
export const On = () => <Login {...sample} auth2faType="on" />;
export const Success = () => <LoginSuccess />;
export const FailedDefault = () => <LoginFailed />;
export const FailedCustom = () => <LoginFailed message="custom message" />;

const sample: State = {
  attempt: {
    isProcessing: false,
    isFailed: false,
    isSuccess: true,
    message: '',
  },
  onLogin: () => null,
  onLoginWithWebauthn: () => null,
  onLoginWithSso: () => null,
  authProviders: [],
  auth2faType: 'off',
  preferredMfaType: 'webauthn',
  isLocalAuthEnabled: true,
  clearAttempt: () => null,
  isPasswordlessEnabled: false,
  primaryAuthType: 'local',
  privateKeyPolicyEnabled: false,
  motd: '',
  showMotd: false,
  acknowledgeMotd: () => null,
};
