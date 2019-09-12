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
import $ from 'jQuery';
import { storiesOf } from '@storybook/react'
import UserResetDialog from './UserResetDialog';

const dialogProps = {
  siteId: 'siteId',
  userId: 'john@smith.com',
  attempt: { },
  attemptActions: { },
  onReset: () => {
    return $.Deferred().reject(new Error('server error'))
  },
  onClose: () => null
}

storiesOf('Gravity/Users/UserResetDialog', module)
  .add('UserResetDialog', () => (
    <UserResetDialog {...dialogProps} />
  ))
  .add('Reset Linkr', () => (
    <UserResetDialog
      { ...dialogProps }
      attempt={{isSuccess: true}}
      resetLink="https://localhost/sample/link"
    />
  ))
  .add('Error', () => (
    <UserResetDialog
      { ...dialogProps }
      attempt={{isFailed: true, message: "server error"}}
    />
  ));