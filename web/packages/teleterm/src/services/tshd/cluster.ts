/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { TrustedDeviceRequirement } from 'gen-proto-ts/teleport/legacy/types/trusted_device_requirement_pb';
import {
  Cluster,
  LoggedInUser_UserType,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';

/**
 * Accepts a proxy host in the form of "cluster-address.example.com:3090" and returns the host as
 * understood by browsers.
 *
 * The URL API in most browsers skips the port if the port matches the default port used by the
 * protocol. This behavior can be observed both in JS and in DOM. For example:
 *
 *     <a href="https://example.com:443/hello-world">Example</a>
 *
 * becomes
 *
 *     <a href="https://example.com/hello-world">Example</a>
 *
 * The distinction is important in situations where we want to match the host reported by the
 * browser against a host that we got from a Go service.
 */
export function proxyHostToBrowserProxyHost(proxyHost: string) {
  let whatwgURL: whatwg.URL;

  try {
    whatwgURL = new whatwg.URL(`https://${proxyHost}`);
  } catch (error) {
    if (error instanceof TypeError) {
      throw new Error(`Invalid proxy host ${proxyHost}`, { cause: error });
    }
    throw error;
  }

  // Catches cases where proxyHost itself includes a "https://" prefix.
  if (whatwgURL.pathname !== '/') {
    throw new Error(`Invalid proxy host ${proxyHost}`);
  }

  return whatwgURL.host;
}

export function proxyHostname(proxyHost: string) {
  let whatwgURL: whatwg.URL;

  try {
    whatwgURL = new whatwg.URL(`https://${proxyHost}`);
  } catch {
    return proxyHost;
  }

  return whatwgURL.hostname;
}

/**
 * Creates a cluster by merging properties that can be read from
 * the profile from disk with properties from details.
 *
 * Listing all fields ensures TypeScript will force us to handle new fields
 * by specifying whether they come from the profile or the details.
 */
export function mergeClusterProfileWithDetails({
  profile,
  details,
}: {
  profile: Cluster;
  details: Cluster;
}): Cluster {
  return {
    uri: profile.uri,
    connected: profile.connected,
    leaf: profile.leaf,
    profileStatusError: profile.profileStatusError,
    proxyHost: profile.proxyHost,
    ssoHost: profile.ssoHost,
    name: details.name,
    showResources: details.showResources,
    features: details.features,
    authClusterId: details.authClusterId,
    proxyVersion: details.proxyVersion,
    loggedInUser: profile.loggedInUser && {
      name: profile.loggedInUser.name,
      activeRequests: profile.loggedInUser.activeRequests,
      roles: profile.loggedInUser.roles,
      isDeviceTrusted: profile.loggedInUser.isDeviceTrusted,
      userType:
        details.loggedInUser?.userType || LoggedInUser_UserType.UNSPECIFIED,
      trustedDeviceRequirement:
        details.loggedInUser?.trustedDeviceRequirement ||
        TrustedDeviceRequirement.UNSPECIFIED,
      requestableRoles: details.loggedInUser?.requestableRoles || [],
      suggestedReviewers: details.loggedInUser?.suggestedReviewers || [],
      acl: details.loggedInUser?.acl || undefined,
    },
  };
}
