/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { getErrorMessage } from 'shared/utils/error';

import { ApiError } from '../api/parseError';

const ProxyRequiresUpgrade = 'Ensure all proxies are upgraded';

/**
 * Determines if error 404 is a result of an endpoint path not found
 * (not resource not found).
 */
export function isPathNotFoundError(err: unknown): err is ApiError {
  if (err instanceof ApiError && err.response.status === 404) {
    if (err.proxyVersion) {
      return true;
    }
  }

  return false;
}

/**
 * Throws a custom error that are a result of `path not found` as a generic
 * error message about this request not being supported and suggests
 * to user to upgrade all proxies to the specified version.
 *
 * Else, rethrows the same error.
 *
 * @param supportedVersion the minimum version required for this
 * request to succeed eg: 17.3.0
 */
export function withGenericUnsupportedError(
  err: unknown,
  supportedVersion: string
): never {
  if (isPathNotFoundError(err)) {
    if (err.proxyVersion) {
      throw new Error(makeUnsupportedErrorMessage(err, supportedVersion));
    }
  }
  throw err;
}

/**
 * `makeUnsupportedErrorMessage` creates a message to the user to explain a
 * version mismatch.
 *
 * @param err the ApiError received
 * @param supportedVersion the minimum version required for this request to
 * succeed eg: 17.3.0
 * @returns the error message to display to the user
 */
function makeUnsupportedErrorMessage(err: ApiError, supportedVersion?: string) {
  const formattedProxyVersion = err.proxyVersion
    ? `(v${err.proxyVersion.string}) `
    : '';
  const formattedSupportedVersion = supportedVersion
    ? `(v${supportedVersion}) `
    : '';
  return (
    `We could not complete your request. Your proxy ${formattedProxyVersion}` +
    `may be behind the minimum required version ${formattedSupportedVersion}` +
    `to support this request. ${ProxyRequiresUpgrade} and try again.`
  );
}

export function withUnsupportedLabelFeatureErrorConversion(
  err: unknown
): never {
  if (err instanceof ApiError && err.response.status === 404) {
    throw new Error(
      'We could not complete your request. ' +
        'Your proxy may be behind the minimum required version ' +
        `(v17.2.0) to support adding resource labels. ` +
        `${ProxyRequiresUpgrade} or remove labels and try again.`
    );
  }
  throw err;
}

export const withUnsupportedOktaPluginUpdateErrorConversion = (
  err: unknown
) => {
  if (err instanceof ApiError && err.response.status === 404) {
    const msg = getErrorMessage(err);
    throw new Error(
      `Could not update Okta plugin: ${msg}. Your proxy may be behind the minimum required version (v17.3.0) to support Okta plugin updates with this web client.`
    );
  }
  throw err;
};

export const withUnsupportedOktaPluginCreateErrorConversion = (
  err: unknown
) => {
  if (err instanceof ApiError) {
    const msg = getErrorMessage(err);
    if (msg.match(/missing okta (?:organization url|api token)/gi)) {
      throw new Error(
        `Could not create Okta plugin: ${msg}. Your proxy may be behind the minimum required version (v17.3.0) to support Okta plugin creation with this web client.`
      );
    }
  }
  throw err;
};
