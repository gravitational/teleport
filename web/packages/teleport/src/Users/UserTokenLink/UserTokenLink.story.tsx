/**
 * Copyright 2020 Gravitational, Inc.
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

import React from 'react';

import Dialog from './UserTokenLink';

export default {
  title: 'Teleport/Users/UserTokenLink',
};

export const Invite = () => {
  return <Dialog {...props} asInvite={true} />;
};

export const Reset = () => {
  return <Dialog {...props} />;
};

const props = {
  token: {
    value: '0c536179038b386728dfee6602ca297f',
    expires: new Date('2021-04-08T07:30:00Z'),
    username: 'Lester',
  },

  onClose() {},
};
