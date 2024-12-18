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

import { MemoryRouter } from 'react-router';

import { SearchResource } from 'teleport/Discover/SelectResource';

import AgentButtonAdd, { Props } from './AgentButtonAdd';

export default {
  title: 'Teleport/AgentButtonAdd',
};

export const CanCreate = () => (
  <MemoryRouter>
    <AgentButtonAdd {...props} />
  </MemoryRouter>
);

export const CannotCreate = () => (
  <MemoryRouter>
    <AgentButtonAdd {...props} canCreate={false} />
  </MemoryRouter>
);

export const CannotCreateVowel = () => (
  <MemoryRouter>
    <AgentButtonAdd
      {...props}
      agent={SearchResource.APPLICATION}
      beginsWithVowel={true}
      canCreate={false}
    />
  </MemoryRouter>
);

export const OnLeaf = () => (
  <MemoryRouter>
    <AgentButtonAdd {...props} isLeafCluster={true} />
  </MemoryRouter>
);

export const OnLeafVowel = () => (
  <MemoryRouter>
    <AgentButtonAdd {...props} isLeafCluster={true} beginsWithVowel={true} />
  </MemoryRouter>
);

const props: Props = {
  agent: SearchResource.SERVER,
  beginsWithVowel: false,
  canCreate: true,
  isLeafCluster: false,
  onClick: () => null,
};
