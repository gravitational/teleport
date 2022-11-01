/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { makeChangedUserAuthn } from './make';

test('makeChangedUserAuthn with null', async () => {
  expect(makeChangedUserAuthn(null)).toStrictEqual({
    recovery: { codes: [], createdDate: null },
    privateKeyPolicyEnabled: false,
  });
});

test('makeChangedUserAuthn with recovery codes', async () => {
  const date = '2022-10-25T00:30:18.162Z';
  expect(
    makeChangedUserAuthn({
      recovery: {
        codes: ['llama', 'alpca'],
        created: date,
      },
      privateKeyPolicyEnabled: true,
    })
  ).toStrictEqual({
    recovery: {
      codes: ['llama', 'alpca'],
      createdDate: new Date('2022-10-25T00:30:18.162Z'),
    },
    privateKeyPolicyEnabled: true,
  });
});
