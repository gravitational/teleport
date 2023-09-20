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
