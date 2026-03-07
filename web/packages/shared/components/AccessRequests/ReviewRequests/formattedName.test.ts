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

import { Resource } from 'shared/services/accessRequests';

import { formattedName } from './formattedName';

function makeResource(
  name: string,
  opts: { subResourceName?: string; friendlyName?: string } = {}
): Resource {
  return {
    id: {
      kind: 'node',
      name,
      clusterName: 'cluster',
      subResourceName: opts.subResourceName,
    },
    details: opts.friendlyName
      ? { friendlyName: opts.friendlyName }
      : undefined,
  };
}

test('returns id.name when no friendlyName is set', () => {
  const resource = makeResource('1234abcd-1234-abcd-1234-abcd1234abcd');
  expect(formattedName(resource)).toBe('1234abcd-1234-abcd-1234-abcd1234abcd');
});

test('returns friendlyName when set, instead of UUID', () => {
  const resource = makeResource('1234abcd-1234-abcd-1234-abcd1234abcd', {
    friendlyName: 'my-hostname',
  });
  expect(formattedName(resource)).toBe('my-hostname');
});

test('includes subResourceName with friendlyName as parent', () => {
  const resource = makeResource('kube-cluster-uuid', {
    subResourceName: 'my-namespace',
    friendlyName: 'kube-cluster-name',
  });
  expect(formattedName(resource)).toBe('kube-cluster-name/my-namespace');
});

test('includes subResourceName with id.name as parent when no friendlyName', () => {
  const resource = makeResource('kube-cluster-uuid', {
    subResourceName: 'my-namespace',
  });
  expect(formattedName(resource)).toBe('kube-cluster-uuid/my-namespace');
});
