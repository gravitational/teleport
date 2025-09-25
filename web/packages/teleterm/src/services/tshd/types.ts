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

/*
 *
 * Do not add new imports to this file, we're trying to get rid of types.ts files.
 *
 */

export {
  /**
   * @deprecated Import directly from gen-proto-ts instead.
   */
  Cluster,
  /**
   * @deprecated Import directly from gen-proto-ts instead.
   */
  LoggedInUser,
  /**
   * @deprecated Import directly from gen-proto-ts instead.
   */
  LoggedInUser_UserType,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';
export {
  /**
   * @deprecated Import directly from gen-proto-ts instead.
   */
  Database,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/database_pb';
export {
  /**
   * @deprecated Import directly from gen-proto-ts instead.
   */
  Gateway,
  /**
   * @deprecated Import directly from gen-proto-ts instead.
   */
  GatewayCLICommand,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/gateway_pb';
export {
  /**
   * @deprecated Import directly from gen-proto-ts instead.
   */
  Server,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/server_pb';
export {
  /**
   * @deprecated Import directly from gen-proto-ts instead.
   */
  Kube,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/kube_pb';
export {
  /**
   * @deprecated Import directly from gen-proto-ts instead.
   */
  Label,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/label_pb';
export {
  /**
   * @deprecated Import directly from gen-proto-ts instead.
   */
  AccessRequest,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/access_request_pb';
export {
  /**
   * @deprecated Import directly from gen-proto-ts instead.
   */
  AccessList,
} from 'gen-proto-ts/teleport/accesslist/v1/accesslist_pb';

// There's too many re-exports from this file to list them individually.
// A @deprecated annotation like this Unfortunately has no effect on the language server.
/**
 * @deprecated Import directly from gen-proto-ts instead.
 */
export * from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb';
