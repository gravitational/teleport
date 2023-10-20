/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
import * as whatwg from 'whatwg-url';

import {
  TELEPORT_CUSTOM_PROTOCOL,
  DeepLinkParsedUri,
  routing,
} from 'teleterm/ui/uri';

export type DeepLinkParseResult =
  // Just having a field like `ok: true` for success and `status: 'error'` for errors would be much more
  // ergonomic. Unfortunately, `if (!result.ok)` doesn't narrow down the type properly with
  // strictNullChecks off. https://github.com/microsoft/TypeScript/issues/10564
  | { status: 'success'; parsedUri: DeepLinkParsedUri }
  | ParseError<'malformed-url', { error: TypeError }>
  | ParseError<'unknown-protocol', { protocol: string }>
  | ParseError<'unsupported-uri'>;

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
 * parseDeepLink receives a full URL of a deep link passed to Connect, e.g.
 * teleport:///clusters/foo/connect_my_computer and returns its parsed form if the underlying URI is
 * supported by the app.
 *
 * Returning a parsed form was a conscious decision – this way it's clear that the parsed form is
 * valid and can be passed along safely from the main process to the renderer vs raw string URLs
 * which don't carry any information by themselves about their validity – in that scenario, they'd
 * have to be parsed on both ends.
 */
export function parseDeepLink(rawUrl: string): DeepLinkParseResult {
  let url: whatwg.URL;
  try {
    url = new whatwg.URL(rawUrl);
  } catch (error) {
    if (error instanceof TypeError) {
      // Invalid URL.
      return { status: 'error', reason: 'malformed-url', error };
    }
    throw error;
  }

  if (url.protocol !== `${TELEPORT_CUSTOM_PROTOCOL}:`) {
    return {
      status: 'error',
      reason: 'unknown-protocol',
      protocol: url.protocol,
    };
  }

  const uri = url.pathname + url.search;
  const parsedUri = routing.parseDeepLinkUri(uri);

  if (!parsedUri) {
    return { status: 'error', reason: 'unsupported-uri' };
  }

  return { status: 'success', parsedUri };
}
