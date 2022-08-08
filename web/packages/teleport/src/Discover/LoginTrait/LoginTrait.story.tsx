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

import React from 'react';

import { LoginTrait } from './LoginTrait';
import { State } from './useLoginTrait';

export default {
  title: 'Teleport/Discover/LoginTrait',
};

export const LoadedWithStatic = () => <LoginTrait {...props} />;

export const LoadedWithoutStatic = () => (
  <LoginTrait {...props} staticLogins={[]} />
);

export const EmptyWithStatic = () => (
  <LoginTrait {...props} dynamicLogins={[]} />
);

export const EmptyWithoutStatic = () => (
  <LoginTrait {...props} dynamicLogins={[]} staticLogins={[]} />
);

export const Processing = () => (
  <LoginTrait {...props} attempt={{ status: 'processing' }} />
);

export const Failed = () => (
  <LoginTrait
    {...props}
    attempt={{ status: 'failed', statusText: 'some error message' }}
  />
);

const props: State = {
  attempt: {
    status: 'success',
    statusText: '',
  },
  dynamicLogins: [
    'root',
    'llama',
    'george_washington_really_long_name_testing',
  ],
  staticLogins: ['ubunutu', 'debian'],
  nextStep: () => null,
  addLogin: () => null,
};
