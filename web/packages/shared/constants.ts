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

/*
 * Be wary that this file is being loaded by both Node.js and browser environments, so keep the
 * definitions simple and do not put environment-specific code here.
 *
 * If it turns out that the design package needs to depend on a constant from here, move this file
 * to the design package or just its own package. This avoids cyclic dependencies as the shared
 * package depends on the design package.
 */

/**
 * TeleportNamespace is used as the namespace prefix for labels defined by Teleport which can
 * carry metadata such as cloud AWS account or instance. Those labels can be used for RBAC.
 *
 * If a label with this prefix is used in a config file, the associated feature must take into
 * account that the label might be removed, modified or could have been set by the user.
 *
 * See also teleport.internal and teleport.hidden.
 */
export const TeleportNamespace = 'teleport.dev' as const;

/**
 * ConnectMyComputerNodeOwnerLabel is a label used to control access to the node managed by
 * Teleport Connect as part of Connect My Computer.
 */
export const ConnectMyComputerNodeOwnerLabel =
  `${TeleportNamespace}/connect-my-computer/owner` as const;
