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

/** Teleport role in a resource format. */
export type RoleResource = Resource<KindRole>;

/**
 * Teleport role in full format.
 * TODO(bl-nero): Add all fields supported on the UI side.
 */
export type Role = {
  kind: KindRole;
  metadata: {
    name: string;
    description?: string;
    labels?: Record<string, string>;
    expires?: string;
    revision?: string;
  };
  spec: {
    allow: RoleConditions;
    deny: RoleConditions;
    options: object;
  };
  version: string;
};

type RoleConditions = {
  node_labels?: Record<string, string[]>;
};

export type RoleWithYaml = {
  object: Role;
  /**
   * yaml string used with yaml editors.
   */
  yaml: string;
};
