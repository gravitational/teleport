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

export type Resource<T extends Kind> = {
  id: string;
  kind: T;
  name: string;
  description?: string;
  // content is config in yaml format.
  content: string;
};

export type KindRole = 'role';
export type KindTrustedCluster = 'trusted_cluster';
export type KindAuthConnectors = 'github' | 'saml' | 'oidc';
export type KindJoinToken = 'join_token';
export type Kind =
  | KindRole
  | KindTrustedCluster
  | KindAuthConnectors
  | KindJoinToken;

/** Describes a Teleport role. */
export type RoleResource = Resource<KindRole>;
