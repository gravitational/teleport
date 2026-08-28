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

/**
 * NodeOwnerLabel is a label used to control access to the node managed by Teleport Connect as part
 * of Connect My Computer. The agent is configured to have this label and the value of it is the
 * username of the cluster user that is running the agent.
 */
export const NodeOwnerLabel = `teleport.dev/connect-my-computer/owner`;

export const getRoleNameForUser = (username: string) =>
  `connect-my-computer-${username}`;
