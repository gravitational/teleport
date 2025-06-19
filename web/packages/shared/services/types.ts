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

export type SSOType =
  | 'microsoft'
  | 'github'
  | 'bitbucket'
  | 'google'
  | 'openid'
  | 'okta'
  | 'unknown';

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

/** Values are taken from https://github.com/gravitational/teleport/blob/0460786b4c3afced1350dd9362ce761806e1c99d/api/types/constants.go#L140-L154 */
export type NodeSubKind = 'teleport' | 'openssh' | 'openssh-ec2-ice';

/** AppSubKind defines names of SubKind for App resource. */
export enum AppSubKind {
  AwsIcAccount = 'aws_ic_account',
  MCP = 'mcp',
}
