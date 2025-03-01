/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { SelectResourceSpec } from '../resources';
import {
  a_DatabaseAws,
  c_ApplicationGcp,
  e_KubernetesSelfHosted_unguided,
  f_Server,
  l_DesktopAzure,
  t_Application_NoAccess,
} from '../testUtils';
import { filterResources, Filters } from './filters';

const resources: SelectResourceSpec[] = [
  c_ApplicationGcp,
  t_Application_NoAccess,
  a_DatabaseAws,
  l_DesktopAzure,
  e_KubernetesSelfHosted_unguided,
  f_Server,
];

describe('filters by resource types', () => {
  const testCases: {
    name: string;
    filter: Filters;
    expected: SelectResourceSpec[];
  }[] = [
    {
      name: 'no filter',
      filter: { resourceTypes: [], hostingPlatforms: [] },
      expected: resources,
    },
    {
      name: 'filter by application',
      filter: { resourceTypes: ['app'], hostingPlatforms: [] },
      expected: [c_ApplicationGcp, t_Application_NoAccess],
    },
    {
      name: 'filter by database',
      filter: { resourceTypes: ['db'], hostingPlatforms: [] },
      expected: [a_DatabaseAws],
    },
    {
      name: 'filter by desktop',
      filter: { resourceTypes: ['desktops'], hostingPlatforms: [] },
      expected: [l_DesktopAzure],
    },
    {
      name: 'filter by kuberenetes',
      filter: { resourceTypes: ['kube'], hostingPlatforms: [] },
      expected: [e_KubernetesSelfHosted_unguided],
    },
    {
      name: 'filter by server',
      filter: { resourceTypes: ['server'], hostingPlatforms: [] },
      expected: [f_Server],
    },
    {
      name: 'filter by server and app',
      filter: { resourceTypes: ['app', 'server'], hostingPlatforms: [] },
      expected: [c_ApplicationGcp, t_Application_NoAccess, f_Server],
    },
  ];
  test.each(testCases)('$name', tc => {
    expect(filterResources(resources, tc.filter)).toEqual(tc.expected);
  });
});

describe('filters by hosting platform', () => {
  const testCases: {
    name: string;
    filter: Filters;
    expected: SelectResourceSpec[];
  }[] = [
    {
      name: 'no filter',
      filter: { resourceTypes: [], hostingPlatforms: [] },
      expected: resources,
    },
    {
      name: 'filter by aws',
      filter: { resourceTypes: [], hostingPlatforms: ['aws'] },
      expected: [a_DatabaseAws],
    },
    {
      name: 'filter by azure',
      filter: { resourceTypes: [], hostingPlatforms: ['azure'] },
      expected: [l_DesktopAzure],
    },
    {
      name: 'filter by gcp',
      filter: { resourceTypes: [], hostingPlatforms: ['gcp'] },
      expected: [c_ApplicationGcp],
    },
    {
      name: 'filter by self-hosted',
      filter: { resourceTypes: [], hostingPlatforms: ['self-hosted'] },
      expected: [e_KubernetesSelfHosted_unguided],
    },
    {
      name: 'filter by aws and azure',
      filter: { resourceTypes: [], hostingPlatforms: ['aws', 'azure'] },
      expected: [a_DatabaseAws, l_DesktopAzure],
    },
  ];
  test.each(testCases)('$name', tc => {
    expect(filterResources(resources, tc.filter)).toEqual(tc.expected);
  });
});

describe('filters by resource types and hosting platform', () => {
  const testCases: {
    name: string;
    filter: Filters;
    expected: SelectResourceSpec[];
  }[] = [
    {
      name: 'no results found',
      filter: { resourceTypes: ['app'], hostingPlatforms: ['aws'] },
      expected: [],
    },
    {
      name: 'filter by app and gcp',
      filter: { resourceTypes: ['app'], hostingPlatforms: ['gcp'] },
      expected: [c_ApplicationGcp],
    },
    {
      name: 'filter by app, kube and self-hosted',
      filter: {
        resourceTypes: ['app', 'kube'],
        hostingPlatforms: ['self-hosted'],
      },
      expected: [e_KubernetesSelfHosted_unguided],
    },
  ];
  test.each(testCases)('$name', tc => {
    expect(filterResources(resources, tc.filter)).toEqual(tc.expected);
  });
});
