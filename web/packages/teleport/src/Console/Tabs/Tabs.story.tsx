/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { TestLayout } from './../Console.story';
import Tabs from './Tabs';

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
    latency: {
      client: 0,
      server: 0,
    },
  } as const,
  {
    id: 22,
    title: 'emwonepu',
    type: 'terminal',
    kind: 'nodes',
    url: 'localhost',
    created: new Date('2019-05-13T20:18:09Z'),
    latency: {
      client: 0,
      server: 0,
    },
  } as const,
  {
    id: 23,
    title: 'foguzaziw',
    type: 'terminal',
    kind: 'nodes',
    url: 'localhost',
    created: new Date('2019-05-13T20:18:09Z'),
    latency: {
      client: 0,
      server: 0,
    },
  } as const,
];
