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

import { GitServer } from 'teleport/services/gitservers';

export const gitServers: GitServer[] = [
  {
    kind: 'git_server',
    id: '00000000-0000-0000-0000-000000000000',
    clusterId: 'im-a-cluster',
    hostname: 'my-org.github-org',
    subKind: 'github',
    labels: [],
    github: {
      organization: 'my-org',
      integration: 'my-org',
    },
  },
];
