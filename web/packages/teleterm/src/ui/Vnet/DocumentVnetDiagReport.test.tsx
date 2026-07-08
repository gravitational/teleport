/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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
  ConnectionStat,
  RecentConnectionKind,
} from 'gen-proto-ts/teleport/lib/teleterm/vnet/v1/vnet_service_pb';

import { filterAndSortConnectionStats } from './DocumentVnetDiagReport';

const makeStat = (overrides: Partial<ConnectionStat>): ConnectionStat => ({
  kind: RecentConnectionKind.APP,
  cluster: 'root',
  leafCluster: '',
  displayName: '',
  port: 0,
  successfulConnections: 0n,
  failedConnections: 0n,
  bytesTx: 0n,
  bytesRx: 0n,
  bytesTxPerSec: 0n,
  bytesRxPerSec: 0n,
  ...overrides,
});

test('sorts by address, then kind, then port', () => {
  const ssh = makeStat({
    kind: RecentConnectionKind.SSH,
    displayName: 'server01.example.com',
  });
  const apiTcp = makeStat({ displayName: 'api.example.com' });
  const grafana8080 = makeStat({
    displayName: 'grafana.example.com',
    port: 8080,
  });
  const grafana3000 = makeStat({
    displayName: 'grafana.example.com',
    port: 3000,
  });

  const result = filterAndSortConnectionStats(
    [grafana8080, ssh, grafana3000, apiTcp],
    ''
  );

  expect(result).toEqual([apiTcp, grafana3000, grafana8080, ssh]);
});

test('filters by address substring, case-insensitive', () => {
  const grafana = makeStat({ displayName: 'grafana.example.com' });
  const api = makeStat({ displayName: 'api.example.com' });

  expect(filterAndSortConnectionStats([grafana, api], 'GRAF')).toEqual([
    grafana,
  ]);
});

test('does not mutate the input array', () => {
  const input = [
    makeStat({ displayName: 'b.example.com' }),
    makeStat({ displayName: 'a.example.com' }),
  ];
  const snapshot = [...input];

  filterAndSortConnectionStats(input, '');

  expect(input).toEqual(snapshot);
});
