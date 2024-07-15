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

import { Resource } from 'gen-proto-ts/teleport/userpreferences/v1/onboard_pb';

import { MarketingParams } from 'teleport/services/userPreferences/types';

import { getMarketingTermMatches } from './getMarketingTermMatches';

describe('getMarketingMatches', () => {
  const testCases: {
    name: string;
    param: MarketingParams;
    expected: Resource[];
  }[] = [
    {
      name: 'database matches DATABASES & k8s matches KUBERNETES',
      param: {
        campaign: 'foodatabasebar',
        source: 'k8ski',
        medium: '',
        intent: '',
      },
      expected: [Resource.DATABASES, Resource.KUBERNETES],
    },
    {
      name: 'app matches WEB_APPLICATIONS',
      param: {
        campaign: '',
        source: 'baz',
        medium: '',
        intent: 'fooappbar',
      },
      expected: [Resource.WEB_APPLICATIONS],
    },
    {
      name: 'windows matches WINDOWS_DESKTOPS',
      param: {
        campaign: 'foowindowsbar',
        source: '',
        medium: '',
        intent: 'aws',
      },
      expected: [Resource.WINDOWS_DESKTOPS],
    },
    {
      name: 'desktop matches WINDOWS_DESKTOPS',
      param: {
        campaign: '',
        source: '',
        medium: 'foodesktopbar',
        intent: 'shoo',
      },
      expected: [Resource.WINDOWS_DESKTOPS],
    },
    {
      name: 'ssh matches SERVER_SSH',
      param: {
        campaign: '',
        source: 'foosshbar',
        medium: 'bar',
        intent: '',
      },
      expected: [Resource.SERVER_SSH],
    },
    {
      name: 'server matches SERVER_SSH',
      param: {
        campaign: 'fooserverbar',
        source: '',
        medium: '',
        intent: 'ser',
      },
      expected: [Resource.SERVER_SSH],
    },
    {
      name: 'kube matches KUBERNETES and windows matches WINDOWS_DESKTOPS',
      param: {
        campaign: 'fookubebar',
        source: '',
        medium: 'windows',
        intent: '',
      },
      expected: [Resource.KUBERNETES, Resource.WINDOWS_DESKTOPS],
    },
    {
      name: 'kubernetes matches KUBERNETES',
      param: {
        campaign: 'kubernetes',
        source: '',
        medium: '',
        intent: '',
      },
      expected: [Resource.KUBERNETES],
    },
    {
      name: 'kube matches KUBERNETES',
      param: {
        campaign: '',
        source: 'kube',
        medium: '',
        intent: '',
      },
      expected: [Resource.KUBERNETES],
    },
    {
      name: 'k8s matches KUBERNETES and ssh matches SERVER_SSH',
      param: {
        campaign: '',
        source: '',
        medium: 'fook8sbar',
        intent: 'ssh',
      },
      expected: [Resource.KUBERNETES, Resource.SERVER_SSH],
    },
    {
      name: 'aws does not match',
      param: {
        campaign: 'fooaws',
        source: 'aws',
        medium: 'awshoo',
        intent: 'no aws',
      },
      expected: [],
    },
    {
      name: 'does not match when empty',
      param: {
        campaign: '',
        source: '',
        medium: '',
        intent: '',
      },
      expected: [],
    },
  ];
  test.each(testCases)('$name', testCase => {
    expect(getMarketingTermMatches(testCase.param)).toEqual(testCase.expected);
  });
});
