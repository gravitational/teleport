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

import { Params, routing } from './uri';

describe('getServerUri', () => {
  const tests: Array<
    { name: string; input: Params } & (
      | { output: string; wantErr?: never }
      | { wantErr: any; output?: never }
    )
  > = [
    {
      name: 'returns a server URI for a root cluster',
      input: { rootClusterId: 'foo', serverId: 'ubuntu' },
      output: '/clusters/foo/servers/ubuntu',
    },
    {
      name: 'returns a server URI for a leaf cluster',
      input: { rootClusterId: 'foo', leafClusterId: 'bar', serverId: 'ubuntu' },
      output: '/clusters/foo/leaves/bar/servers/ubuntu',
    },
    {
      name: 'throws an error if serverId is missing from the root cluster URI',
      input: { rootClusterId: 'foo' },
      wantErr: new TypeError('Expected "serverId" to be defined'),
    },
    {
      name: 'throws an error if serverId is missing from the leaf cluster URI',
      input: { rootClusterId: 'foo', leafClusterId: 'bar' },
      wantErr: new TypeError('Expected "serverId" to be defined'),
    },
    {
      // This isn't necessarily a behavior which we should depend on, but we should document it
      // nonetheless.
      name: 'returns a server URI if extra params are included',
      input: { rootClusterId: 'foo', serverId: 'ubuntu', dbId: 'postgres' },
      output: '/clusters/foo/servers/ubuntu',
    },
  ];

  /* eslint-disable jest/no-conditional-expect */
  test.each(tests)('$name', ({ input, output, wantErr }) => {
    if (wantErr) {
      expect(() => routing.getServerUri(input)).toThrow(wantErr);
    } else {
      expect(routing.getServerUri(input)).toEqual(output);
    }
  });
  /* eslint-enable jest/no-conditional-expect */
});

describe('getKubeResourceNamespaceUri', () => {
  const tests: Array<{ name: string; input: Params } & { output: string }> = [
    {
      name: 'returns a kube resource namespace URI for a root cluster',
      input: {
        rootClusterId: 'foo',
        kubeId: 'kubeClusterName',
        kubeNamespaceId: 'namespace',
      },
      output: '/clusters/foo/kubes/kubeClusterName/namespaces/namespace',
    },
    {
      name: 'returns a kube resource namespace URI for a leaf cluster',
      input: {
        rootClusterId: 'foo',
        leafClusterId: 'bar',
        kubeId: 'kubeClusterName',
        kubeNamespaceId: 'namespace',
      },
      output:
        '/clusters/foo/leaves/bar/kubes/kubeClusterName/namespaces/namespace',
    },
  ];

  /* eslint-disable jest/no-conditional-expect */
  test.each(tests)('$name', ({ input, output }) => {
    expect(routing.getKubeResourceNamespaceUri(input)).toEqual(output);
  });
  /* eslint-enable jest/no-conditional-expect */
});
