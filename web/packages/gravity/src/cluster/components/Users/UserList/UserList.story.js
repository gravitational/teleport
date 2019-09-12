/*
Copyright 2019 Gravitational, Inc.

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

import React from 'react'
import { storiesOf } from '@storybook/react'
import UserList from './UserList'

storiesOf('Gravity/Users/UserList', module)
  .add('UserList', () => {
    return (
      <UserList users={users} roleLabels={roleLabels} />
    );
  });

const roleLabels = [
  '@teleadmin',
  'admin',
  'Mehsovu',
  'Agnetif',
  'Duonluv',
  'Surolel',
  'Uparouj',
  'Ohuzadgec', 'Hohaca', 'Afutipjaw', 'Uzununo', ]

const users = [
  {
    "userId": "ijemali@example.com",
    "isNew": false,
    "name": "",
    "email": "ijemali@example.com",
    "status": "active",
    "builtin": false,
    "created": "2019-01-29T16:25:12.084Z",
    "roles": [ "@teleadmin", "admin", "Ohuzadgec", "Hohaca", "Afutipjaw", "Uzununo" ],
    "owner": false
  },
  {
    "userId": "baober@example.com",
    "isNew": false,
    "name": "",
    "email": "redebofan@example.com",
    "status": "active",
    "builtin": false,
    "created": "2019-02-13T20:22:01.057Z",
    "roles": [ "@teleadmin", "admin", "Ohuzadgec", "Hohaca", "Afutipjaw", "Uzununo" ],
    "owner": false
  },
  {
    "userId": "kicoges@example.com",
    "isNew": false,
    "name": "",
    "email": "nimjozen@example.com",
    "status": "active",
    "builtin": false,
    "created": "2019-01-14T18:19:04.463Z",
    "roles": [
      "@teleadmin"
    ],
    "owner": false
  },
]