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

import {
  AuthenticateWebDeviceDeepURL,
  ConnectMyComputerDeepURL,
  CUSTOM_PROTOCOL,
  DeepURL,
  Path,
} from 'shared/deepLinks';

export type DeepLinkParseResult =
  // Just having a field like `ok: true` for success and `status: 'error'` for errors would be much more
  // ergonomic. Unfortunately, `if (!result.ok)` doesn't narrow down the type properly with
  // strictNullChecks off. https://github.com/microsoft/TypeScript/issues/10564
  | DeepLinkParseResultSuccess
  | ParseError<'malformed-url', { error: TypeError }>
  | ParseError<'unknown-protocol', { protocol: string }>
  | ParseError<'unsupported-url'>;

export type DeepLinkParseResultSuccess = {
  status: 'success';
  url: DeepURL;
};

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
 * pathname is the path from the URL with the leading slash included, e.g. if the URL is
 * "teleport://example.com/connect_my_computer", the pathname is "/connect_my_computer"
 */

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

  const { host, hostname, port, username, pathname, searchParams } = whatwgURL;
  const baseUrl = {
    host,
    hostname,
    port,
    // whatwg-url percent-encodes usernames. We decode them here so that the rest of the app doesn't
    // have to do this. https://url.spec.whatwg.org/#set-the-username
    username: decodeURIComponent(username),
  };

  switch (pathname as Path) {
    case '/connect_my_computer': {
      const url: ConnectMyComputerDeepURL = {
        ...baseUrl,
        pathname: '/connect_my_computer',
        searchParams: {},
      };
      return { status: 'success', url };
    }
    case '/authenticate_web_device': {
      const id = searchParams.get('id');
      const token = searchParams.get('token');
      const redirect_uri = searchParams.get('redirect_uri');
      if (!(id && token)) {
        return {
          status: 'error',
          reason: 'malformed-url',
          error: new TypeError(
            'id and token must be included in the deep link for authenticating a web device'
          ),
        };
      }

      const url: AuthenticateWebDeviceDeepURL = {
        ...baseUrl,
        pathname: '/authenticate_web_device',
        searchParams: {
          id,
          token,
          redirect_uri,
        },
      };
      return { status: 'success', url };
    }
    default:
      return { status: 'error', reason: 'unsupported-url' };
  }
}
