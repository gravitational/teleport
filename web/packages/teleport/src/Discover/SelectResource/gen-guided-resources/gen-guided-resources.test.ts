/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { SelectResourceSpec } from 'teleport/Discover/SelectResource/resources';
import {
  DatabaseLocation,
  KubeLocation,
} from 'teleport/Discover/SelectResource/types';
import { ResourceKind } from 'teleport/Discover/Shared/ResourceKind';

import {
  createGuidedResourceList,
  kindLabelPlural,
} from './gen-guided-resources';

// makeResource returns a minimal SelectResourceSpec for use in tests. Only the
// fields accessed by the generator functions need to be populated.
function makeResource(
  overrides: Partial<SelectResourceSpec>
): SelectResourceSpec {
  return {
    id: undefined,
    kind: ResourceKind.Server,
    name: 'Test Resource',
    keywords: [],
    icon: 'linux',
    event: undefined,
    ...overrides,
  } as unknown as SelectResourceSpec;
}

describe('kindLabelPlural', () => {
  const testCases = [
    { kind: ResourceKind.Application, expected: 'Applications' },
    { kind: ResourceKind.Database, expected: 'Databases' },
    { kind: ResourceKind.Desktop, expected: 'Desktops' },
    { kind: ResourceKind.Kubernetes, expected: 'Kubernetes' },
    { kind: ResourceKind.Server, expected: 'Servers' },
    { kind: ResourceKind.ConnectMyComputer, expected: 'Connect My Computer' },
    { kind: ResourceKind.SamlApplication, expected: 'SAML Applications' },
    { kind: ResourceKind.MCP, expected: 'MCP Servers' },
  ];

  test.each(testCases)(
    'returns "$expected" for ResourceKind.$kind',
    ({ kind, expected }) => {
      expect(kindLabelPlural(kind)).toEqual(expected);
    }
  );
});

describe('createGuidedResourceList', () => {
  test('formats guided resources as MDX sections grouped by type', () => {
    const resources = [
      makeResource({ name: 'PostgreSQL', kind: ResourceKind.Database }),
      makeResource({ name: 'EKS', kind: ResourceKind.Kubernetes }),
    ];

    const expected = `### Databases

| Resource | Deployment Type |
|----------|----------|
| PostgreSQL | N/A |

### Kubernetes

| Resource | Deployment Type |
|----------|----------|
| EKS | N/A |`;
    expect(createGuidedResourceList(resources)).toEqual(expected);
  });

  test('sorts rows by type then name', () => {
    const resources = [
      makeResource({ name: 'Ubuntu 18.04+', kind: ResourceKind.Server }),
      makeResource({ name: 'PostgreSQL', kind: ResourceKind.Database }),
      makeResource({ name: 'Amazon Linux 2/2023', kind: ResourceKind.Server }),
      makeResource({ name: 'EKS', kind: ResourceKind.Kubernetes }),
      makeResource({ name: 'MySQL/MariaDB', kind: ResourceKind.Database }),
    ];

    const expected = `### Databases

| Resource | Deployment Type |
|----------|----------|
| MySQL/MariaDB | N/A |
| PostgreSQL | N/A |

### Kubernetes

| Resource | Deployment Type |
|----------|----------|
| EKS | N/A |

### Servers

| Resource | Deployment Type |
|----------|----------|
| Amazon Linux 2/2023 | SSH |
| Ubuntu 18.04+ | SSH |`;
    expect(createGuidedResourceList(resources)).toEqual(expected);
  });

  test('excludes resources with unguidedLink', () => {
    const resources = [
      makeResource({ name: 'EKS', kind: ResourceKind.Kubernetes }),
      makeResource({
        name: 'MongoDB',
        kind: ResourceKind.Database,
        unguidedLink:
          'https://goteleport.com/docs/enroll-resources/database-access/enroll-self-hosted-databases/mongodb-self-hosted',
      }),
    ];

    const expected = `### Kubernetes

| Resource | Deployment Type |
|----------|----------|
| EKS | N/A |`;
    expect(createGuidedResourceList(resources)).toEqual(expected);
  });

  test('excludes Connect My Computer resources', () => {
    const resources = [
      makeResource({ name: 'Ubuntu 18.04+', kind: ResourceKind.Server }),
      makeResource({
        name: 'Connect My Computer',
        kind: ResourceKind.ConnectMyComputer,
      }),
    ];

    const expected = `### Servers

| Resource | Deployment Type |
|----------|----------|
| Ubuntu 18.04+ | SSH |`;
    expect(createGuidedResourceList(resources)).toEqual(expected);
  });

  test('populates the Deployment Type column from resource metadata', () => {
    const resources = [
      makeResource({
        name: 'RDS PostgreSQL',
        kind: ResourceKind.Database,
        dbMeta: { location: DatabaseLocation.Aws } as any,
      }),
      makeResource({
        name: 'PostgreSQL',
        kind: ResourceKind.Database,
        dbMeta: { location: DatabaseLocation.SelfHosted } as any,
      }),
      makeResource({
        name: 'EKS',
        kind: ResourceKind.Kubernetes,
        kubeMeta: { location: KubeLocation.Aws },
      }),
    ];

    const expected = `### Databases

| Resource | Deployment Type |
|----------|----------|
| PostgreSQL | Self-Hosted |
| RDS PostgreSQL | Amazon Web Services (AWS) |

### Kubernetes

| Resource | Deployment Type |
|----------|----------|
| EKS | Amazon Web Services (AWS) |`;
    expect(createGuidedResourceList(resources)).toEqual(expected);
  });

  test('returns just the header for empty input', () => {
    const expected = ``;
    expect(createGuidedResourceList([])).toEqual(expected);
  });
});
