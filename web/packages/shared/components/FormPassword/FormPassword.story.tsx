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

import FormPassword from './FormPassword';

export default {
  title: 'Shared/FormPassword',
};

export const Off = () => <FormPassword {...props} />;

export const OTP = () => <FormPassword auth2faType="otp" {...props} />;

export const Webauthn = () => (
  <FormPassword auth2faType="webauthn" {...props} />
);

export const On = () => (
  <FormPassword auth2faType="on" {...props} preferredMfaType="webauthn" />
);

export const Optional = () => (
  <FormPassword auth2faType="optional" {...props} preferredMfaType="webauthn" />
);

const props = {
  onChangePass: () => Promise.resolve(),
  onChangePassWithWebauthn: () => Promise.reject(new Error('server error')),
};
