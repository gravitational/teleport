/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { Desktop } from 'teleport/services/desktops';

export const desktops: Desktop[] = [
  {
    os: 'windows',
    name: 'bb8411a4-ba50-537c-89b3-226a00447bc6',
    addr: 'host.com',
    labels: [{ name: 'foo', value: 'bar' }],
    logins: ['Administrator'],
  },
  {
    os: 'windows',
    name: 'd96e7dd6-26b6-56d5-8259-778f943f90f2',
    addr: 'another.com',
    labels: [],
    logins: ['Administrator'],
  },
  {
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
