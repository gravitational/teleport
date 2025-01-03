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

/**
 * The version of the proxy where the error occurred.
 *
 * Currently, the proxy version field is only returned
 * with api routes "not found" error.
 *
 * Used to determine outdated proxies.
 *
 * This response was introduced in v17.2.0.
 */
interface ProxyVersion {
  major: number;
  minor: number;
  patch: number;
  /**
   * defined if version is not for production eg:
   * the prerelease value for version 17.0.0-dev, is "dev"
   */
  preRelease: string;
  /**
   * full version in string eg: "17.0.0-dev"
   */
  string: string;
}

interface ApiErrorConstructor {
  /**
   * message is the main error, usually the "cause" of the error.
   */
  message: string;
  response: Response;
  proxyVersion?: ProxyVersion;
  opts?: ErrorOptions;
  messages?: string[];
}

export default function parseError(json) {
  let msg = '';

  if (json && json.error) {
    msg = json.error.message;
  } else if (json && json.message) {
    msg = json.message;
  } else if (json.responseText) {
    msg = json.responseText;
  }
  return msg;
}

export function parseProxyVersion(json): ProxyVersion | undefined {
  return json?.fields?.proxyVersion;
}

export class ApiError extends Error {
  response: Response;
  /**
   * messages contains a list of other user related errors
   * aside from the main error set for the field `[Error].message`.
   *
   * messages is part of the Trace error object as well, where each
   * time an error is wrapped with trace.Wrap, a new message gets
   * added to messages.
   */
  messages: string[];

  /**
   * Only defined with api routes "not found" error.
   */
  proxyVersion?: ProxyVersion;

  constructor({
    message,
    response,
    proxyVersion,
    opts,
    messages,
  }: ApiErrorConstructor) {
    message = message || 'Unknown error';
    super(message, opts);
    this.response = response;
    this.name = 'ApiError';
    this.messages = messages || [];
    this.proxyVersion = proxyVersion;
  }
}
