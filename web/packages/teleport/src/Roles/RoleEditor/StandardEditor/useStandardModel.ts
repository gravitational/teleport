/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { Dispatch, useReducer } from 'react';

import { Role } from 'teleport/services/resources';

import {
  hasModifiedFields,
  MetadataModel,
  newResourceAccess,
  newRole,
  newRuleModel,
  OptionsModel,
  ResourceAccess,
  ResourceAccessKind,
  RoleEditorModel,
  roleToRoleEditorModel,
  RuleModel,
  StandardEditorModel,
} from './standardmodel';
import { validateRoleEditorModel } from './validation';

/**
 * Creates a standard model state and returns an array composed of the state
 * and an action dispatcher that can be used to change it.
 */
export const useStandardModel = (
  originalRole?: Role
): [StandardEditorModel, StandardModelDispatcher] =>
  useReducer(reduce, originalRole, initializeState);

const initializeState = (originalRole?: Role): StandardEditorModel => {
  const role = originalRole ?? newRole();
  const roleModel = roleToRoleEditorModel(role, role);
  return {
    roleModel,
    originalRole,
    isDirty: !originalRole, // New role is dirty by default.
    validationResult: validateRoleEditorModel(roleModel, undefined, undefined),
  };
};

/** A function for dispatching model-changing actions. */
export type StandardModelDispatcher = Dispatch<StandardModelAction>;

type StandardModelAction =
  | SetModelAction
  | SetMetadataAction
  | AddResourceAccessAction
  | SetResourceAccessAction
  | RemoveResourceAccessAction
  | AddAccessRuleAction
  | SetAccessRuleAction
  | RemoveAccessRuleAction
  | SetOptionsAction;

/** Sets the entire model. */
type SetModelAction = { type: 'set-role-model'; payload: RoleEditorModel };
type SetMetadataAction = { type: 'set-metadata'; payload: MetadataModel };
type AddResourceAccessAction = {
  type: 'add-resource-access';
  payload: { kind: ResourceAccessKind };
};
type SetResourceAccessAction = {
  type: 'set-resource-access';
  payload: ResourceAccess;
};
type RemoveResourceAccessAction = {
  type: 'remove-resource-access';
  payload: { kind: ResourceAccessKind };
};
type AddAccessRuleAction = { type: 'add-access-rule'; payload?: never };
type SetAccessRuleAction = { type: 'set-access-rule'; payload: RuleModel };
type RemoveAccessRuleAction = {
  type: 'remove-access-rule';
  payload: { id: string };
};
type SetOptionsAction = { type: 'set-options'; payload: OptionsModel };

/** Produces a new model using existing state and the action. */
const reduce = (
  state: StandardEditorModel,
  action: StandardModelAction
): StandardEditorModel => {
  // We need to give `type` a different name or the assertion in the `default`
  // block will cause a syntax error.
  const { type, payload } = action;

  switch (type) {
    case 'set-role-model':
      return updateRoleModel(state, payload);

    case 'set-metadata':
      return updateRoleModel(state, { metadata: payload });

    case 'set-options':
      return updateRoleModel(state, { options: payload });

    case 'add-resource-access':
      return updateRoleModel(state, {
        resources: [
          ...state.roleModel.resources,
          newResourceAccess(payload.kind),
        ],
      });

    case 'set-resource-access':
      return updateRoleModel(state, {
        resources: state.roleModel.resources.map(r =>
          r.kind === payload.kind ? payload : r
        ),
      });

    case 'remove-resource-access':
      return updateRoleModel(state, {
        resources: state.roleModel.resources.filter(
          r => r.kind !== payload.kind
        ),
      });

    case 'add-access-rule':
      return updateRoleModel(state, {
        rules: [...state.roleModel.rules, newRuleModel()],
      });

    case 'set-access-rule':
      return updateRoleModel(state, {
        rules: state.roleModel.rules.map(r =>
          r.id === payload.id ? payload : r
        ),
      });

    case 'remove-access-rule':
      return updateRoleModel(state, {
        rules: state.roleModel.rules.filter(r => r.id !== payload.id),
      });

    default:
      (type) satisfies never;
      return state;
  }
};

/**
 * Creates a new model state given existing state and a patch to
 * RoleEditorModel. Validates the state and recognizes whether it's dirty (i.e.
 * changed from the original).
 */
const updateRoleModel = (
  { roleModel, originalRole, validationResult }: StandardEditorModel,
  roleModelPatch: Partial<RoleEditorModel>
): StandardEditorModel => {
  const newRoleModel = { ...roleModel, ...roleModelPatch };
  return {
    roleModel: newRoleModel,
    originalRole,
    isDirty: hasModifiedFields(newRoleModel, originalRole),
    validationResult: validateRoleEditorModel(
      newRoleModel,
      roleModel,
      validationResult
    ),
  };
};
