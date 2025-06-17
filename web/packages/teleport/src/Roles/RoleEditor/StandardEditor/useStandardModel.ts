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

import { current, enableMapSet, original } from 'immer';
import { Dispatch } from 'react';
import { useImmerReducer } from 'use-immer';

import { Logger } from 'design/logger';

import { Role, RoleVersion, Verb } from 'teleport/services/resources';

import {
  hasModifiedFields,
  MetadataModel,
  newResourceAccess,
  newRole,
  newRuleModel,
  newVerbsModel,
  OptionsModel,
  ResourceAccess,
  ResourceAccessKind,
  ResourceKindOption,
  RoleEditorModel,
  roleToRoleEditorModel,
  StandardEditorModel,
  StandardEditorTab,
} from './standardmodel';
import { validateRoleEditorModel } from './validation';

const logger = new Logger('useStandardModel');

// Enable support for the Set type in Immer. We use it for `disabledTabs`.
enableMapSet();

/**
 * Creates a standard model state and returns an array composed of the state
 * and an action dispatcher that can be used to change it. Since the conversion
 * is a complex process, we put additional protection against unexpected errors
 * here: if an error is thrown, the {@link StandardEditorModel.roleModel} and
 * {@link StandardEditorModel.validationResult} will be set to `undefined`.
 */
export const useStandardModel = (
  originalRole?: Role
): [StandardEditorModel, StandardModelDispatcher] =>
  useImmerReducer(reduce, originalRole, initializeState);

const initializeState = (originalRole?: Role): StandardEditorModel => {
  const isEditing = !!originalRole;
  const role = originalRole ?? newRole();
  const roleModel = safelyConvertRoleToEditorModel(role);
  return {
    roleModel,
    originalRole,
    isDirty: !originalRole, // New role is dirty by default.
    isTouched: false,
    validationResult:
      roleModel && validateRoleEditorModel(roleModel, undefined, undefined),
    currentTab: StandardEditorTab.Overview,
    disabledTabs: isEditing
      ? new Set()
      : new Set([
          StandardEditorTab.Resources,
          StandardEditorTab.AdminRules,
          StandardEditorTab.Options,
        ]),
  };
};

const safelyConvertRoleToEditorModel = (
  role: Role
): RoleEditorModel | undefined => {
  try {
    return roleToRoleEditorModel(role, role);
  } catch (err) {
    logger.error('Could not convert Role to a standard model', err);
    return undefined;
  }
};

/** A function for dispatching model-changing actions. */
export type StandardModelDispatcher = Dispatch<StandardModelAction>;

export enum ActionType {
  SetCurrentTab = 'SetCurrentTab',
  SetModel = 'SetModel',
  ResetToStandard = 'ResetToStandard',
  SetMetadata = 'SetMetadata',
  SetRoleNameCollision = 'SetRoleNameCollision',
  AddResourceAccess = 'AddResourceAccess',
  SetResourceAccess = 'SetResourceAccess',
  RemoveResourceAccess = 'RemoveResourceAccess',
  AddAdminRule = 'AddAdminRule',
  SetAdminRuleResources = 'SetAdminRuleResources',
  SetAdminRuleVerb = 'SetAdminRuleVerb',
  SetAdminRuleAllVerbs = 'SetAdminRuleAllVerbs',
  SetAdminRuleWhere = 'SetAdminRuleWhere',
  RemoveAdminRule = 'RemoveAdminRule',
  SetOptions = 'SetOptions',
  EnableValidation = 'EnableValidation',
}

type StandardModelAction =
  | SetCurrentTabAction
  | SetModelAction
  | ResetToStandardAction
  | SetMetadataAction
  | SetRoleNameCollisionAction
  | AddResourceAccessAction
  | SetResourceAccessAction
  | RemoveResourceAccessAction
  | AddAdminRuleAction
  | SetAdminRuleResourcesAction
  | SetAdminRuleVerbAction
  | SetAdminRuleAllVerbsAction
  | SetAdminRuleWhereAction
  | RemoveAdminRuleAction
  | SetOptionsAction
  | EnableValidationAction;

/** Sets the entire model. */
type SetCurrentTabAction = {
  type: ActionType.SetCurrentTab;
  payload: StandardEditorTab;
};
type SetModelAction = { type: ActionType.SetModel; payload: RoleEditorModel };
type ResetToStandardAction = {
  type: ActionType.ResetToStandard;
  payload?: never;
};
type SetMetadataAction = {
  type: ActionType.SetMetadata;
  payload: MetadataModel;
};
type SetRoleNameCollisionAction = {
  type: ActionType.SetRoleNameCollision;
  payload: boolean;
};
type AddResourceAccessAction = {
  type: ActionType.AddResourceAccess;
  payload: { kind: ResourceAccessKind };
};
type SetResourceAccessAction = {
  type: ActionType.SetResourceAccess;
  payload: ResourceAccess;
};
type RemoveResourceAccessAction = {
  type: ActionType.RemoveResourceAccess;
  payload: { kind: ResourceAccessKind };
};
type AddAdminRuleAction = { type: ActionType.AddAdminRule; payload?: never };
type SetAdminRuleResourcesAction = {
  type: ActionType.SetAdminRuleResources;
  payload: { id: string; resources: readonly ResourceKindOption[] };
};
type SetAdminRuleVerbAction = {
  type: ActionType.SetAdminRuleVerb;
  payload: { id: string; verb: Verb; checked: boolean };
};
type SetAdminRuleAllVerbsAction = {
  type: ActionType.SetAdminRuleAllVerbs;
  payload: { id: string; checked: boolean };
};
type SetAdminRuleWhereAction = {
  type: ActionType.SetAdminRuleWhere;
  payload: { id: string; where: string };
};
type RemoveAdminRuleAction = {
  type: ActionType.RemoveAdminRule;
  payload: { id: string };
};
type SetOptionsAction = { type: ActionType.SetOptions; payload: OptionsModel };
type EnableValidationAction = {
  type: ActionType.EnableValidation;
  payload?: never;
};

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
    case ActionType.SetCurrentTab:
      state.currentTab = payload;
      state.disabledTabs.delete(payload);
      break;

    case ActionType.SetModel:
      state.roleModel = payload;
      state.isTouched = false;
      break;

    case ActionType.ResetToStandard:
      state.roleModel.conversionErrors = [];
      state.roleModel.requiresReset = false;
      state.isTouched = true;
      break;

    case ActionType.SetMetadata:
      state.roleModel.metadata = payload;
      updateRoleVersionInResources(
        state.roleModel.resources,
        payload.version.value
      );
      state.isTouched = true;
      break;

    case ActionType.SetRoleNameCollision:
      state.roleModel.metadata.nameCollision = payload;
      break;

    case ActionType.SetOptions:
      state.roleModel.options = payload;
      state.isTouched = true;
      break;

    case ActionType.AddResourceAccess:
      state.roleModel.resources.push(
        newResourceAccess(payload.kind, state.roleModel.metadata.version.value)
      );
      state.isTouched = true;
      break;

    case ActionType.SetResourceAccess:
      state.roleModel.resources = state.roleModel.resources.map(r =>
        r.kind === payload.kind ? payload : r
      );
      state.isTouched = true;
      break;

    case ActionType.RemoveResourceAccess:
      state.roleModel.resources = state.roleModel.resources.filter(
        r => r.kind !== payload.kind
      );
      state.isTouched = true;
      break;

    case ActionType.AddAdminRule:
      state.roleModel.rules.push(newRuleModel());
      state.isTouched = true;
      break;

    case ActionType.SetAdminRuleResources: {
      const rule = state.roleModel.rules.find(r => r.id === payload.id);
      rule.resources = payload.resources;
      // Update the verbs, as with changing resources, the list of allowed
      // verbs may also change.
      const newVerbs = newVerbsModel(rule.resources);
      for (const nv of newVerbs) {
        if (rule.allVerbs) {
          // If the "All" checkbox is selected, just keep everything checked.
          nv.checked = true;
        } else {
          // Otherwise, copy from the current state.
          const currentVerb = rule.verbs.find(cv => cv.verb == nv.verb);
          if (currentVerb) {
            nv.checked = currentVerb.checked;
          }
        }
      }
      rule.verbs = newVerbs;
      state.isTouched = true;
      break;
    }

    case ActionType.SetAdminRuleVerb: {
      const rule = state.roleModel.rules.find(r => r.id === payload.id);
      rule.verbs.find(v => v.verb === payload.verb).checked = payload.checked;
      if (!payload.checked) {
        rule.allVerbs = false;
      }
      state.isTouched = true;
      break;
    }

    case ActionType.SetAdminRuleAllVerbs: {
      const rule = state.roleModel.rules.find(r => r.id === payload.id);
      rule.allVerbs = payload.checked;
      // When we check the "All" checkbox, all verbs should be checked
      // immediately. When we uncheck the "All" checkbox, all verbs should be
      // cleared.
      for (const v of rule.verbs) {
        v.checked = payload.checked;
      }
      state.isTouched = true;
      break;
    }

    case ActionType.SetAdminRuleWhere:
      state.roleModel.rules.find(r => r.id === payload.id).where =
        payload.where;
      state.isTouched = true;
      break;

    case ActionType.RemoveAdminRule:
      state.roleModel.rules = state.roleModel.rules.filter(
        r => r.id !== payload.id
      );
      state.isTouched = true;
      break;

    case ActionType.EnableValidation:
      for (const r of state.roleModel.resources) {
        r.hideValidationErrors = false;
      }
      for (const r of state.roleModel.rules) {
        r.hideValidationErrors = false;
      }
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
