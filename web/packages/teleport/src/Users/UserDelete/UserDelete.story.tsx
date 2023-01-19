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

import { UserDelete } from './UserDelete';

export default {
  title: 'Teleport/Users/UserDelete',
};

export const Processing = () => {
  return <UserDelete {...props} attempt={{ status: 'processing' }} />;
};

export const Confirm = () => {
  return <UserDelete {...props} attempt={{ status: '' }} />;
};

export const Failed = () => {
  return (
    <UserDelete
      {...props}
      attempt={{ status: 'failed', statusText: 'server error' }}
    />
  );
};

const props = {
  username: 'somename',
  onDelete: () => null,
  onClose: () => null,
};
