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

export type SecondFactor = 'otp' | 'webauthn' | 'sso';

// legacySecondFactorToSecondFactors converts the legacy second_factor field into the
// equivalent list of second_factors.
// TODO(Joerger): DELETE IN v20 - v19 server sets second_factors.
export function legacySecondFactorToSecondFactors(
  secondFactor: Auth2faType
): SecondFactor[] {
  switch (secondFactor) {
    case 'otp':
      return ['otp'];
    case 'webauthn':
      return ['webauthn'];
    case 'on':
    case 'optional':
      return ['otp', 'webauthn'];
    default:
      return [];
  }
}

// secondFactorsToLegacySecondFactor converts a list of second_factors into the
// equivalent legacy second_factor field.
//
// The conversion is lossy:
// - [sso] -> "off"
// - [sso, x] -> "otp" | "webauthn" | "on"
// - No combination of second_factors can result in "optional"
//
// TODO(Joerger): the 'optional' auth2faType is currently never sent by v17+ servers
// due to a bug - https://github.com/gravitational/teleport/issues/67274. As a result,
// we can accept the loss of "optional" in this conversion for the time being. If we decide
// to fix "optional", this conversion can be updated to check an 'is_mfa_optional' field.
export function secondFactorsToLegacySecondFactor(
  secondFactors: SecondFactor[]
): Auth2faType {
  const otp = secondFactors.includes('otp');
  const webauthn = secondFactors.includes('webauthn');

  if (otp && webauthn) {
    return 'on';
  }
  if (webauthn) {
    return 'webauthn';
  }
  if (otp) {
    return 'otp';
  }
  return 'off';
}

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

/**
 * AppProtocol defines the protocol of an App resource.
 */
export type AppProtocol = 'TCP' | 'HTTP' | 'MCP';
