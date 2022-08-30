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

import React from 'react';

import DocumentBlank from './DocumentBlank';
import { TestLayout } from './../Console.story';

export default {
  title: 'Teleport/Console/DocumentBlank',
};

export const Blank = () => (
  <TestLayout>
    <DocumentBlank
      visible={true}
      doc={
        {
          created: new Date(),
          kind: 'blank',
          url: '',
          clusterId: 'one',
        } as const
      }
    />
  </TestLayout>
);
