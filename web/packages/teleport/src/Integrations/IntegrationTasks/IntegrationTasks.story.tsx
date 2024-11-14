/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import React from 'react';
import { MemoryRouter } from 'react-router';

import { Tasks } from './IntegrationTasks';

export default {
  title: 'Teleport/Integrations/Tasks/Aws',
};

export const WithTasksOpensFirstTaskInList = () => (
  <MemoryRouter>
    <Tasks />
  </MemoryRouter>
);

export const WithoutTasks = () => (
  <MemoryRouter>
    <Tasks />
  </MemoryRouter>
);

export const MarkAsResolvedFailed = () => (
  <MemoryRouter>
    <Tasks />
  </MemoryRouter>
);

export const FailedFetch = () => (
  <MemoryRouter>
    <Tasks />
  </MemoryRouter>
);

export const Loading = () => (
  <MemoryRouter>
    <Tasks />
  </MemoryRouter>
);

// Sidbar scrolling with failed instances list gets long, the top action bar should be sticky
