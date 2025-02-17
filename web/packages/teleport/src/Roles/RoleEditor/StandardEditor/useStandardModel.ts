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

import { current, original } from 'immer';
import { Dispatch } from 'react';
import { useImmerReducer } from 'use-immer';

import { Role, RoleVersion } from 'teleport/services/resources';

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
  useImmerReducer(reduce, originalRole, initializeState);

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
  | ResetToStandardAction
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
type ResetToStandardAction = { type: 'reset-to-standard'; payload?: never };
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

  // This reduce uses Immer, so we modify the model draft directly.
  // TODO(bl-nero): add immutability to the model data types.
  switch (type) {
    case 'set-role-model':
      state.roleModel = payload;
      break;

    case 'reset-to-standard':
      state.roleModel.conversionErrors = [];
      state.roleModel.requiresReset = false;
      break;

    case 'set-metadata':
      state.roleModel.metadata = payload;
      updateRoleVersionInResources(
        state.roleModel.resources,
        payload.version.value
      );
      break;

    case 'set-options':
      state.roleModel.options = payload;
      break;

    case 'add-resource-access':
      state.roleModel.resources.push(
        newResourceAccess(payload.kind, state.roleModel.metadata.version.value)
      );
      break;

    case 'set-resource-access':
      state.roleModel.resources = state.roleModel.resources.map(r =>
        r.kind === payload.kind ? payload : r
      );
      break;

    case 'remove-resource-access':
      state.roleModel.resources = state.roleModel.resources.filter(
        r => r.kind !== payload.kind
      );
      break;

    case 'add-access-rule':
      state.roleModel.rules.push(newRuleModel());
      break;

    case 'set-access-rule':
      state.roleModel.rules = state.roleModel.rules.map(r =>
        r.id === payload.id ? payload : r
      );
      break;

    case 'remove-access-rule':
      state.roleModel.rules = state.roleModel.rules.filter(
        r => r.id !== payload.id
      );
      break;

    default:
      (type) satisfies never;
      return state;
  }

  processEditorModel(state);
};

/**
 * Recomputes dependent fields of a state draft. Validates the state and
 * recognizes whether it's dirty (i.e. changed from the original).
 */
const processEditorModel = (state: StandardEditorModel) => {
  const { roleModel, originalRole, validationResult } = state;
  state.isDirty = hasModifiedFields(roleModel, originalRole);
  state.validationResult = validateRoleEditorModel(
    // It's crucial to use `current` and `original` here from the performance
    // standpoint, since `validateEditorModel` recognizes unchanged state by
    // reference. We want to make sure that the objects passed to it have
    // stable identities.
    current(state).roleModel,
    original(state).roleModel,
    validationResult
  );
};

const updateRoleVersionInResources = (
  resources: ResourceAccess[],
  version: RoleVersion
) => {
  for (const res of resources.filter(res => res.kind === 'kube_cluster')) {
    res.roleVersion = version;
    for (const kubeRes of res.resources) {
      kubeRes.roleVersion = version;
    }
  }
};
