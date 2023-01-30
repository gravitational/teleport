/*
Copyright 2019-2020 Gravitational, Inc.

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

import Tabs from './Tabs';
import { TestLayout } from './../Console.story';

export default {
  title: 'Teleport/Console',
};

export const ConsoleTabs = () => (
  <TestLayout>
    <Tabs
      width="100%"
      height="32px"
      activeTab={35}
      items={items}
      parties={parties}
      disableNew={false}
      onNew={() => null}
      onSelect={() => null}
    />
  </TestLayout>
);

ConsoleTabs.storyName = 'Tabs';

const parties = {
  '16': [{ user: 'jiawu@ninu.fo' }, { user: 'gigbu@fe.ac' }],
  '15': [{ user: 'difzu@gusebdem.gq' }, { user: 'tuicu@bu.ki' }],
  '27': [],
};

const items = [
  {
    id: 35,
    title: 'tuguwog',
    type: 'terminal',
    kind: 'nodes',
    url: 'localhost',
    created: new Date('2019-05-13T20:18:09Z'),
  } as const,
  {
    id: 22,
    title: 'emwonepu',
    type: 'terminal',
    kind: 'nodes',
    url: 'localhost',
    created: new Date('2019-05-13T20:18:09Z'),
  } as const,
  {
    id: 23,
    title: 'foguzaziw',
    type: 'terminal',
    kind: 'nodes',
    url: 'localhost',
    created: new Date('2019-05-13T20:18:09Z'),
  } as const,
];
