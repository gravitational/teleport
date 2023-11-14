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

/*
 * This file does not import whatwg-url and thus does not perform any parsing on purpose. whatwg-url
 * provides uniform parsing across browser implementations but it weights a ton [1], so we should
 * avoid pulling it into the deps of Web UI.
 *
 * [1] https://bundlephobia.com/package/whatwg-url@13.0.0
 */

// We're able to get away with defining the path like this only because we don't use matchPath from
// React Router v5 like teleterm/src/ui/uri.ts does. Once we get to more complex use cases that will
// use matchPath, we'll likely have to sacrifice some type safety.
// The underscore was chosen as a separator since resource kinds on the backend already use
// underscores.
export enum Path {
  ConnectMyComputer = 'connect_my_computer',
}

export const CUSTOM_PROTOCOL = 'teleport' as const;

/**
 * makeDeepLinkWithSafeInput creates a deep link by interpolating passed arguments.
 *
 * Important: This function does not perform any validation or parsing. It must not be called with
 * user-generated content.
 */
export function makeDeepLinkWithSafeInput(args: {
  /**
   * proxyHost is the address and an optional port number of the proxy, e.g.
   * "bigco.cloud.gravitational.io" or "teleport-local.dev:3080". In the Web UI proxyHost is
   * commonly referred to as publicURL.
   */
  proxyHost: string;
  path: Path;
  username: string | undefined;
}): string {
  // The username in a URL should be percend-encoded. [1]
  //
  // What's more, Chrome 119, unlike Firefox and Safari, won't even trigger a custom protocol prompt
  // when clicking on a link that includes a username with an @ symbol that is not percent-encoded,
  // e.g. teleport://alice@example.com@example.com/connect_my_computer.
  //
  // [1] https://url.spec.whatwg.org/#set-the-username
  const encodedUsername = args.username
    ? encodeURIComponent(args.username) + '@'
    : '';

  return `${CUSTOM_PROTOCOL}://${encodedUsername}${args.proxyHost}/${args.path}`;
}
