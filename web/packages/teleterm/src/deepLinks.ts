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

import * as whatwg from 'whatwg-url';
import { CUSTOM_PROTOCOL, Path } from 'shared/deepLinks';

export type DeepLinkParseResult =
  // Just having a field like `ok: true` for success and `status: 'error'` for errors would be much more
  // ergonomic. Unfortunately, `if (!result.ok)` doesn't narrow down the type properly with
  // strictNullChecks off. https://github.com/microsoft/TypeScript/issues/10564
  | DeepLinkParseResultSuccess
  | ParseError<'malformed-url', { error: TypeError }>
  | ParseError<'unknown-protocol', { protocol: string }>
  | ParseError<'unsupported-uri'>;

export type DeepLinkParseResultSuccess = { status: 'success'; url: DeepURL };

type ParseError<Reason, AdditionalData = void> = AdditionalData extends void
  ? {
      status: 'error';
      reason: Reason;
    }
  : {
      status: 'error';
      reason: Reason;
    } & AdditionalData;

/**
 *
 * DeepURL is a parsed version of an URL.
 *
 * Since DeepLinkParseResult goes through IPC in Electron [1], anything included in it is subject to
 * Structured Clone Algorithm [2]. As such, getters and setters are dropped which means were not
 * able to pass whatwg.URL without casting it to an object.
 *
 * [1] https://www.electronjs.org/docs/latest/tutorial/ipc
 * [2] https://developer.mozilla.org/en-US/docs/Web/API/Web_Workers_API/Structured_clone_algorithm
 */
export type DeepURL = {
  /**
   * host is the hostname plus the port.
   */
  host: string;
  /**
   * hostname is the host without the port, e.g. if the host is "example.com:4321", the hostname is
   * "example.com".
   */
  hostname: string;
  port: string;
  /**
   * username is percent-decoded username from the URL. whatwg-url encodes usernames found in URLs.
   * parseDeepLink decodes them so that other parts of the app don't have to deal with this.
   */
  username: string;
  /**
   * pathname is the path from the URL with the leading slash included, e.g. if the URL is
   * "teleport://example.com/connect_my_computer", the pathname is "/connect_my_computer"
   */
  pathname: `/${Path}`;
};

/**
 * parseDeepLink receives a full URL of a deep link passed to Connect, e.g.
 * teleport://foo.example.com:4321/connect_my_computer and returns its parsed form if the underlying
 * URI is supported by the app.
 *
 * Returning a parsed form was a conscious decision – this way it's clear that the parsed form is
 * valid and can be passed along safely from the main process to the renderer vs raw string URLs
 * which don't carry any information by themselves about their validity – in that scenario, they'd
 * have to be parsed on both ends.
 */
export function parseDeepLink(rawUrl: string): DeepLinkParseResult {
  let whatwgURL: whatwg.URL;
  try {
    whatwgURL = new whatwg.URL(rawUrl);
  } catch (error) {
    if (error instanceof TypeError) {
      // Invalid URL.
      return { status: 'error', reason: 'malformed-url', error };
    }
    throw error;
  }

  if (whatwgURL.protocol !== `${CUSTOM_PROTOCOL}:`) {
    return {
      status: 'error',
      reason: 'unknown-protocol',
      protocol: whatwgURL.protocol,
    };
  }

  if (whatwgURL.pathname !== '/connect_my_computer') {
    return { status: 'error', reason: 'unsupported-uri' };
  }

  const { host, hostname, port, username, pathname } = whatwgURL;
  const url: DeepURL = {
    host,
    hostname,
    port,
    // whatwg-url percent-encodes usernames. We decode them here so that the rest of the app doesn't
    // have to do this. https://url.spec.whatwg.org/#set-the-username
    username: decodeURIComponent(username),
    pathname,
  };

  return { status: 'success', url };
}
