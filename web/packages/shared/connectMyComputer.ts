/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

/**
 * NodeOwnerLabel is a label used to control access to the node managed by Teleport Connect as part
 * of Connect My Computer. The agent is configured to have this label and the value of it is the
 * username of the cluster user that is running the agent.
 */
export const NodeOwnerLabel = `teleport.dev/connect-my-computer/owner` as const;

export const getRoleNameForUser = (username: string) =>
  `connect-my-computer-${username}`;
