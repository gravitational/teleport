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

import {
  ClusterResource,
  MarketingParams,
} from 'teleport/services/userPreferences/types';

import { getMarketingTermMatches } from './getMarketingTermMatches';

describe('getMarketingMatches', () => {
  const testCases: {
    name: string;
    param: MarketingParams;
    expected: ClusterResource[];
  }[] = [
    {
      name: 'database matches RESOURCE_DATABASES & k8s matches RESOURCE_KUBERNETES',
      param: {
        campaign: 'foodatabasebar',
        source: 'k8ski',
        medium: '',
        intent: '',
      },
      expected: [
        ClusterResource.RESOURCE_DATABASES,
        ClusterResource.RESOURCE_KUBERNETES,
      ],
    },
    {
      name: 'app matches RESOURCE_WEB_APPLICATIONS',
      param: {
        campaign: '',
        source: 'baz',
        medium: '',
        intent: 'fooappbar',
      },
      expected: [ClusterResource.RESOURCE_WEB_APPLICATIONS],
    },
    {
      name: 'windows matches RESOURCE_WINDOWS_DESKTOPS',
      param: {
        campaign: 'foowindowsbar',
        source: '',
        medium: '',
        intent: 'aws',
      },
      expected: [ClusterResource.RESOURCE_WINDOWS_DESKTOPS],
    },
    {
      name: 'desktop matches RESOURCE_WINDOWS_DESKTOPS',
      param: {
        campaign: '',
        source: '',
        medium: 'foodesktopbar',
        intent: 'shoo',
      },
      expected: [ClusterResource.RESOURCE_WINDOWS_DESKTOPS],
    },
    {
      name: 'ssh matches RESOURCE_SERVER_SSH',
      param: {
        campaign: '',
        source: 'foosshbar',
        medium: 'bar',
        intent: '',
      },
      expected: [ClusterResource.RESOURCE_SERVER_SSH],
    },
    {
      name: 'server matches RESOURCE_SERVER_SSH',
      param: {
        campaign: 'fooserverbar',
        source: '',
        medium: '',
        intent: 'ser',
      },
      expected: [ClusterResource.RESOURCE_SERVER_SSH],
    },
    {
      name: 'kube matches RESOURCE_KUBERNETES and windows matches RESOURCE_WINDOWS_DESKTOPS',
      param: {
        campaign: 'fookubebar',
        source: '',
        medium: 'windows',
        intent: '',
      },
      expected: [
        ClusterResource.RESOURCE_KUBERNETES,
        ClusterResource.RESOURCE_WINDOWS_DESKTOPS,
      ],
    },
    {
      name: 'kubernetes matches RESOURCE_KUBERNETES',
      param: {
        campaign: 'kubernetes',
        source: '',
        medium: '',
        intent: '',
      },
      expected: [ClusterResource.RESOURCE_KUBERNETES],
    },
    {
      name: 'kube matches RESOURCE_KUBERNETES',
      param: {
        campaign: '',
        source: 'kube',
        medium: '',
        intent: '',
      },
      expected: [ClusterResource.RESOURCE_KUBERNETES],
    },
    {
      name: 'k8s matches RESOURCE_KUBERNETES and ssh matches RESOURCE_SERVER_SSH',
      param: {
        campaign: '',
        source: '',
        medium: 'fook8sbar',
        intent: 'ssh',
      },
      expected: [
        ClusterResource.RESOURCE_KUBERNETES,
        ClusterResource.RESOURCE_SERVER_SSH,
      ],
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
