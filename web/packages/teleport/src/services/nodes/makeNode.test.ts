/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import makeNode from './makeNode';

describe('makeNode', () => {
  it('parses a full node JSON', () => {
    const node = makeNode({
      id: 'node-1',
      siteId: 'cluster-a',
      subKind: 'teleport',
      hostname: 'host.example.com',
      addr: '1.2.3.4:22',
      tunnel: false,
      tags: [{ name: 'env', value: 'prod' }],
      sshLogins: ['root', 'ubuntu'],
      sshLoginDetails: [
        { login: 'root', requiresRequest: true },
        { login: 'ubuntu', requiresRequest: false },
      ],
      requiresRequest: false,
      supportedFeatureIds: [1],
    });

    expect(node.kind).toBe('node');
    expect(node.id).toBe('node-1');
    expect(node.clusterId).toBe('cluster-a');
    expect(node.hostname).toBe('host.example.com');
    expect(node.sshLogins).toEqual(['root', 'ubuntu']);
    expect(node.sshLoginDetails).toEqual([
      { login: 'root', requiresRequest: true },
      { login: 'ubuntu', requiresRequest: false },
    ]);
    expect(node.supportedFeatureIds).toEqual([1]);
  });

  describe('sshLoginDetails', () => {
    it('returns undefined when sshLoginDetails is nullish', () => {
      const node = makeNode({ sshLoginDetails: null });
      expect(node.sshLoginDetails).toBeUndefined();
    });

    it('defaults login to empty string when missing', () => {
      const node = makeNode({
        sshLoginDetails: [{ requiresRequest: true }],
      });
      expect(node.sshLoginDetails).toEqual([
        { login: '', requiresRequest: true },
      ]);
    });

    it('preserves requiresRequest as-is', () => {
      const node = makeNode({
        sshLoginDetails: [
          { login: 'root', requiresRequest: true },
          { login: 'ubuntu', requiresRequest: false },
          { login: 'admin' },
        ],
      });
      expect(node.sshLoginDetails).toEqual([
        { login: 'root', requiresRequest: true },
        { login: 'ubuntu', requiresRequest: false },
        { login: 'admin', requiresRequest: undefined },
      ]);
    });
  });

  it('returns defaults for nullish input', () => {
    const node = makeNode(null);
    expect(node.kind).toBe('node');
    expect(node.sshLogins).toEqual([]);
    expect(node.sshLoginDetails).toBeUndefined();
    expect(node.labels).toEqual([]);
  });
});
