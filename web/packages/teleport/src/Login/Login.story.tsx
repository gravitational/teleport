/*
Copyright 2019-2021 Gravitational, Inc.

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

export default {
  title: 'Teleport/Login',
};

export const Form = () => <Login {...sample} />;
export const Success = () => <LoginSuccess />;
export const FailedDefault = () => <LoginFailed />;
export const FailedCustom = () => <LoginFailed message="custom message" />;

const sample = {
  attempt: {
    isProcessing: false,
    status: 'success' as any,
    isFailed: false,
    isSuccess: true,
    message: '',
  },
  onLogin: () => null,
  onLoginWithU2f: () => null,
  onLoginWithSso: () => null,
  authProviders: [],
  auth2faType: 'off' as any,
  isLocalAuthEnabled: true,
  clearAttempt: () => null,
};
