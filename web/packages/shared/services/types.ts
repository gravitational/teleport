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

import { privateKeyEnablingPolicies } from './consts';

export type AuthProviderType = 'oidc' | 'saml' | 'github';

export type Auth2faType = 'otp' | 'off' | 'optional' | 'on' | 'webauthn';

export type AuthType = 'local' | 'github' | 'oidc' | 'saml';

// PrimaryAuthType defines types where if:
//  - local: preferred authn is with username and password
//  - passwordless: preferred authn is passwordless
//  - sso: preferred authn is either with github, oidc or saml provider
export type PrimaryAuthType = 'local' | 'passwordless' | 'sso';

// PreferredMfaType is used to determine which MFA option
// is preferred when more than one option can be available
// and only one should be preferred.
//
// DELETE IN 11.0.0, preferredMfaType currently has no usage, other
// than in teleterm, where we check if auth settings return
// the deprecated `u2f` option (v9). Starting v10
// 'u2f' will automatically be aliased to 'webauthn'.
export type PreferredMfaType = 'webauthn' | 'u2f' | '';

export type AuthProvider = {
  displayName?: string;
  name: string;
  type: AuthProviderType;
  url: string;
};

export type PrivateKeyPolicy =
  | 'none'
  | (typeof privateKeyEnablingPolicies)[number];
