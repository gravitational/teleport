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

import { Desktop } from 'teleport/services/desktops';

export const desktops: Desktop[] = [
  {
    kind: 'windows_desktop',
    os: 'windows',
    name: 'bb8411a4-ba50-537c-89b3-226a00447bc6',
    addr: 'host.com',
    labels: [{ name: 'foo', value: 'bar' }],
    logins: ['Administrator'],
  },
  {
    kind: 'windows_desktop',
    os: 'windows',
    name: 'd96e7dd6-26b6-56d5-8259-778f943f90f2',
    addr: 'another.com',
    labels: [],
    logins: ['Administrator'],
  },
  {
    kind: 'windows_desktop',
    os: 'windows',
    name: '18cd6652-2f9a-5475-8138-2a56d44e1645',
    addr: 'yetanother.com',
    labels: [
      { name: 'bar', value: 'foo' },
      { name: 'foo', value: 'bar' },
    ],
    logins: ['Administrator'],
  },
];

export const moreDesktops: Desktop[] = [
  {
    kind: 'windows_desktop',
    os: 'windows',
    name: 'e3b163f6-4ccf-5352-9b80-a9be8327afe8',
    addr: 'host.com',
    labels: [{ name: 'foo', value: 'bar' }],
    logins: ['Administrator'],
  },
  {
    kind: 'windows_desktop',
    os: 'windows',
    name: 'b71f9b28-3774-55a3-a894-1a5e73ad7328',
    addr: 'another.com',
    labels: [],
    logins: ['Administrator'],
  },
  {
    kind: 'windows_desktop',
    os: 'windows',
    name: 'f49cd40d-a967-54e5-99a9-f62727943ca2',
    addr: 'yetanother.com',
    labels: [
      { name: 'bar', value: 'foo' },
      { name: 'foo', value: 'bar' },
    ],
    logins: ['Administrator'],
  },
];
