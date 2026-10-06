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

import { TrustedDeviceRequirement } from 'gen-proto-ts/teleport/legacy/types/trusted_device_requirement_pb';
import { LoggedInUser_UserType } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';

import {
  mergeClusterProfileWithDetails,
  proxyHostToBrowserProxyHost,
} from './cluster';
import { makeAcl, makeLoggedInUser, makeRootCluster } from './testHelpers';

describe('proxyHostToBrowserProxyHost', () => {
  describe('valid inputs', () => {
    const tests: Array<{
      name: string;
      input: string;
      expectedOutput: string;
    }> = [
      {
        name: 'default port',
        input: 'cluster.example.com:443',
        expectedOutput: 'cluster.example.com',
      },
      {
        name: 'custom port',
        input: 'example.com:3090',
        expectedOutput: 'example.com:3090',
      },
      {
        name: 'top-level domain with default port',
        input: 'teleport-local:443',
        expectedOutput: 'teleport-local',
      },
      {
        name: 'top-level domain with custom port',
        input: 'teleport-local:3090',
        expectedOutput: 'teleport-local:3090',
      },
    ];

    test.each(tests)('$name ($input)', ({ input, expectedOutput }) => {
      expect(proxyHostToBrowserProxyHost(input)).toEqual(expectedOutput);
    });
  });

  describe('invalid inputs', () => {
    const tests: Array<{
      name: string;
      input: string;
    }> = [
      {
        name: 'proxyHost includes protocol',
        input: 'https://cluster.example.com:3090',
      },
      {
        name: 'URL parsing error',
        input: '<teleport>',
      },
    ];

    test.each(tests)('$name ($input)', ({ input }) => {
      expect(() => proxyHostToBrowserProxyHost(input)).toThrow(
        /invalid proxy host/i
      );
    });
  });
});

describe('mergeClusterProfileWithDetails', () => {
  const detailsUser = makeLoggedInUser({
    name: 'alice',
    display: { primary: 'Alice Example', secondary: 'alice@example.com' },
    userType: LoggedInUser_UserType.LOCAL,
    trustedDeviceRequirement: TrustedDeviceRequirement.REQUIRED,
    requestableRoles: ['requester'],
    suggestedReviewers: ['reviewer'],
    acl: makeAcl(),
  });

  const keptDetails = {
    display: detailsUser.display,
    userType: detailsUser.userType,
    trustedDeviceRequirement: detailsUser.trustedDeviceRequirement,
    requestableRoles: detailsUser.requestableRoles,
    suggestedReviewers: detailsUser.suggestedReviewers,
    acl: detailsUser.acl,
  };

  const droppedDetails = {
    display: undefined,
    userType: LoggedInUser_UserType.UNSPECIFIED,
    trustedDeviceRequirement: TrustedDeviceRequirement.UNSPECIFIED,
    requestableRoles: [],
    suggestedReviewers: [],
    acl: undefined,
  };

  test.each([
    {
      name: 'matching username',
      profileName: 'alice',
      expectedDetails: keptDetails,
    },
    {
      name: 'changed username',
      profileName: 'bob',
      expectedDetails: droppedDetails,
    },
    {
      name: 'empty username',
      profileName: '',
      expectedDetails: droppedDetails,
    },
  ])('$name', ({ profileName, expectedDetails }) => {
    // Setup: details belong to Alice while the disk profile can change independently.
    const details = makeRootCluster({ loggedInUser: detailsUser });
    const profile = makeRootCluster({
      loggedInUser: makeLoggedInUser({ name: profileName }),
    });

    // Execute: merge the new profile with previously fetched details.
    const merged = mergeClusterProfileWithDetails({ profile, details });

    // Assert: details belong only to the matching username.
    expect({
      display: merged.loggedInUser?.display,
      userType: merged.loggedInUser?.userType,
      trustedDeviceRequirement: merged.loggedInUser?.trustedDeviceRequirement,
      requestableRoles: merged.loggedInUser?.requestableRoles,
      suggestedReviewers: merged.loggedInUser?.suggestedReviewers,
      acl: merged.loggedInUser?.acl,
    }).toEqual(expectedDetails);
  });

  test('keeps the username when details have no display', () => {
    // Setup: details contain no resolved display.
    const profile = makeRootCluster({
      loggedInUser: makeLoggedInUser({ name: 'alice' }),
    });
    const details = makeRootCluster({
      loggedInUser: makeLoggedInUser({ name: 'alice' }),
    });

    // Execute: merge the profile and details.
    const merged = mergeClusterProfileWithDetails({ profile, details });

    // Assert: the username remains and display is absent.
    expect(merged.loggedInUser?.name).toBe('alice');
    expect(merged.loggedInUser?.display).toBeUndefined();
  });
});
