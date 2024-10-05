/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { equalsDeep } from 'shared/utils/highbar';

import { Role } from 'teleport/services/resources';

import { defaultOptions } from './withDefaults';

export type StandardEditorModel = {
  roleModel: RoleEditorModel;
  /**
   * Will be true if fields have been modified from the original.
   */
  isDirty: boolean;
};

/**
 * A temporary representation of the role, reflecting the structure of standard
 * editor UI. Since the standard editor UI structure doesn't directly represent
 * the structure of the role resource, we introduce this intermediate model.
 */
export type RoleEditorModel = {
  metadata: MetadataModel;
  version: string;
  /**
   * Indicates whether the current resource, as described by YAML, is
   * accurately represented by this editor model. If it's not, the user needs
   * to agree to reset it to a compatible resource before editing it in the
   * structured editor.
   */
  requiresReset: boolean;
};

export type MetadataModel = {
  name: string;
  description?: string;
  revision?: string;
};

const roleVersion = 'v7';

/**
 * Returns the role object with required fields defined with empty values.
 */
export function newRole(): Role {
  return {
    kind: 'role',
    metadata: {
      name: 'new_role_name',
    },
    spec: {
      allow: {},
      deny: {},
      options: defaultOptions(),
    },
    version: roleVersion,
  };
}

/**
 * Converts a role to its in-editor UI model representation. The resulting
 * model may be marked as requiring reset if the role contains unsupported
 * features.
 */
export function roleToRoleEditorModel(
  role: Role,
  originalRole?: Role
): RoleEditorModel {
  // We use destructuring to strip fields from objects and assert that nothing
  // has been left. Therefore, we don't want Lint to warn us that we didn't use
  // some of the fields.
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const { kind, metadata, spec, version, ...rest } = role;
  const { name, description, revision, ...mRest } = metadata;
  const { allow, deny, options, ...sRest } = spec;
  return {
    metadata: {
      name,
      description,
      revision,
    },
    version,
    requiresReset:
      revision !== originalRole?.metadata?.revision ||
      version !== roleVersion ||
      !(
        isEmpty(rest) &&
        isEmpty(mRest) &&
        isEmpty(sRest) &&
        isEmpty(allow) &&
        isEmpty(deny) &&
        equalsDeep(options, defaultOptions())
      ),
  };
}

function isEmpty(obj: object) {
  return Object.keys(obj).length === 0;
}

/**
 * Converts a role editor model to a role. This operation is lossless.
 */
export function roleEditorModelToRole(roleModel: RoleEditorModel): Role {
  const { name, description, revision, ...mRest } = roleModel.metadata;
  const { version } = roleModel;
  // Compile-time assert that protects us from silently losing fields.
  mRest satisfies Record<any, never>;

  return {
    kind: 'role',
    metadata: {
      name,
      description,
      revision,
    },
    spec: {
      allow: {},
      deny: {},
      options: defaultOptions(),
    },
    version,
  };
}

/** Detects if fields were modified by comparing against the original role. */
export function hasModifiedFields(
  updated: RoleEditorModel,
  originalRole: Role
) {
  return (
    updated.metadata.name !== originalRole?.metadata.name ||
    updated.metadata.description !== originalRole?.metadata.description
  );
}
