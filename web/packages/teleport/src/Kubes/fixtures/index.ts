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

import { Kube } from 'teleport/services/kube';

export const kubes: Kube[] = [
  {
    kind: 'kube_cluster',
    name: 'tele.logicoma.dev-prod',
    labels: [
      { name: 'kernel', value: '4.15.0-51-generic' },
      { name: 'env', value: 'prod' },
    ],
  },
  {
    kind: 'kube_cluster',
    name: 'tele.logicoma.dev-staging',
    labels: [{ name: 'env', value: 'staging' }],
  },
  {
    kind: 'kube_cluster',
    name: 'cookie',
    labels: [
      { name: 'cluster-name', value: 'some-cluster-name' },
      { name: 'env', value: 'idk' },
    ],
  },
];

export const moreKubes: Kube[] = [
  {
    kind: 'kube_cluster',
    name: 'tele.logicoma.official-dev',
    labels: [
      { name: 'kernel', value: '4.15.0-51-generic' },
      { name: 'env', value: 'official-dev' },
    ],
  },
  {
    kind: 'kube_cluster',
    name: 'tele.logicoma.official-prod',
    labels: [{ name: 'env', value: 'official-prod' }],
  },
  {
    kind: 'kube_cluster',
    name: 'cookie2',
    labels: [
      { name: 'cluster-name', value: 'some-cluster-name' },
      { name: 'env', value: 'idk' },
    ],
  },
];
