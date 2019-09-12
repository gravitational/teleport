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
import { UserInviteDialog } from './UserInviteDialog'

storiesOf('Gravity/Users/UserInviteDialog', module)
  .add('UserInviteDialog', () => {
    return (
      <UserInviteDialog
        attempt={{}}
        open={true}
        roles={roles} />
    );
  })
  .add('With Error', () => {
    return (
      <UserInviteDialog
        attempt={{ isFailed: true, message: serverError }}
        open={true}
        roles={roles} />
    );
  })
  .add('With Invite Link', () => {
    return (
      <UserInviteDialog
        attempt={{ isSuccess: true, message: userToken.url }}
        open={true}
        roles={roles} />
    );
  });

const userToken = {
  url: 'https://172.31.28.130:3009/web/newuser/157220e7d29956398c0722bea4f38825d78de645e92e9f3980f650154957bb59',
}

const serverError = "this is a long error message which should be wrapped";

const roles = [
  'admin', 'devops' , 'segonlog', 'jozfekgon', 'zewibovuk', 'vekredo', 'rebwumu', 'warifif', 'upnihnuj', 'ubkuznav', 'rurlizo'
]