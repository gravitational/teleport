/*
Copyright 2020 Gravitational, Inc.

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
import { Invite as Component } from './Invite';

export default {
  title: 'Teleport/Invite',
  component: Component,
};

export function Invite() {
  return <Component {...defaultProps} />;
}

export function Expired() {
  const props = {
    ...defaultProps,
    fetchAttempt: { isFailed: true, message: 'this is error message' },
  };

  return <Component {...props} />;
}

export function ResetPasswordScreen() {
  const props = {
    ...defaultProps,
    passwordResetMode: true,
  };
  return <Component {...props} />;
}

const defaultProps = {
  auth2faType: 'off',
  submitAttempt: {},
  fetchAttempt: {
    isSuccess: true,
  },
  onSubmitWithU2f: () => null,
  fetchUserToken: () => null,
  onSubmit: () => null,
  passwordToken: {
    user: 'john@example.com',
    url: 'https://localhost/sampleurl',
  },
};
