import cfg from 'e-teleport/config';
import { AccessListPreset } from 'e-teleport/services/accessmanagement/preset';

import {
  AwsIcRoleConditions,
  StandardRoleConditions,
} from '../GuideEditor/Preset/role/conditions';
import { RequiredAppIdentitiesWithFetchResult } from '../GuideEditor/Preset/role/resources/app';
import { Grant, Members, Owners, Spec } from './types';

export function goToCreateAccessListFromOktaRoute(
  oktaOrgUrl: string,
  locState?: ResumableCreateAccessListState
) {
  let state = { oktaOrgUrl };
  if (locState) {
    state = { oktaOrgUrl, ...locState };
  }
  return {
    to: cfg.routes.accessListNew,
    state,
  };
}

/**
 * State required to resume AWS Identity Center role configuration.
 */
export type ResumableAwsIcRoleState = {
  awsIcRoleConditions: AwsIcRoleConditions;
};

/**
 * State required to resume standard role configuration.
 */
export type ResumableStandardRoleState = {
  standardRoleConditions: StandardRoleConditions;
  requiredAppIdentities: RequiredAppIdentitiesWithFetchResult;
};

/**
 * State required to resume the guided editor at the correct step.
 */
export type ResumableGuideEditorState = {
  preset: AccessListPreset;
  resumeStep: number;
  oktaOrgUrl: string;
  // Use the same session event id when resuming a state to help
  // correlate emitted events.
  eventSessionId: string;
};

/**
 * Combined state for resuming the access list guided flow.
 */
export type ResumableGuideState = ResumableAwsIcRoleState &
  ResumableStandardRoleState &
  ResumableGuideEditorState;

/**
 * State required to resume the access list creation form.
 */
export type ResumableAccessListState = {
  spec: Spec;
  owners: Owners;
  ownerGrant: Grant;
  members: Members;
  memberGrant: Grant;
};

/**
 * Complete state for resuming access list creation from any point.
 * This is the full state passed via React Router's location state when
 * navigating to the create access list route.
 */
export type ResumableCreateAccessListState = ResumableGuideState &
  ResumableAccessListState;
