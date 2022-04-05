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

export type AuthProviderType = 'oidc' | 'saml' | 'github';

export type Auth2faType = 'otp' | 'off' | 'optional' | 'on' | 'webauthn';

// PreferredMfaType is used to determine which MFA option
// is preferred when more than one option can be available
// and only one should be preferred.
//
// TODO(lisa) remove, currently does nothing.
export type PreferredMfaType = 'webauthn' | '';

export type AuthProvider = {
  displayName?: string;
  name: string;
  type: AuthProviderType;
  url: string;
};
