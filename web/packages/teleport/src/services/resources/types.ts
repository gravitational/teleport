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
 * Teleport role in full format, as returned from Teleport API.
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
    options: RoleOptions;
  };
  version: string;
};

export type RoleConditions = {
  node_labels?: Labels;
  logins?: string[];
};

export type Labels = Record<string, string | string[]>;

/**
 * Teleport role options in full format, as returned from Teleport API. Note
 * that its fields follow the snake case convention to match the wire format.
 */
export type RoleOptions = {
  cert_format: string;
  create_db_user: boolean;
  create_desktop_user: boolean;
  desktop_clipboard: boolean;
  desktop_directory_sharing: boolean;
  enhanced_recording: string[];
  forward_agent: boolean;
  idp: {
    // There's a subtle quirk in `Rolev6.CheckAndSetDefaults`: if you ask
    // Teleport to create a resource with `idp` field set to null, it's instead
    // going to create the entire idp->saml->enabled structure. However, it's
    // possible to create a role with idp set to an empty object, and the
    // server will retain this state. This makes the `saml` field optional.
    saml?: {
      enabled: boolean;
    };
  };
  max_session_ttl: string;
  pin_source_ip: boolean;
  port_forwarding: boolean;
  record_session: {
    default: string;
    desktop: boolean;
  };
  ssh_file_copy: boolean;
};

export type RoleWithYaml = {
  object: Role;
  /**
   * yaml string used with yaml editors.
   */
  yaml: string;
};
