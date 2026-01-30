import { useState } from 'react';

import { AccessListPreset } from 'e-teleport/services/accessmanagement/preset';
import { RoleVersion } from 'teleport/services/resources';

import {
  AwsIcRoleState,
  useAwsIcRoleState,
} from './Preset/DefineAccess/AwsIc/useAwsIcRoleState';
import {
  StandardRoleState,
  useStandardRoleState,
} from './Preset/DefineAccess/useStandardRoleState';
import {
  definableResourceAccessFields,
  DefinableResourceAccessFields,
} from './Preset/role/listaccess';

/**
 * Do not change.
 *
 * This is the lowest role version that this guide editor supports.
 * It also marks the start of this guide editor feature.
 */
export const MinimumRoleVersionSupported = RoleVersion.V8;

export type GuideEditorState = {
  /**
   * Describes the type of selected preset.
   * Will be initially null to mean "no preset selected".
   */
  preset: AccessListPreset;
  setPreset(kind: AccessListPreset): void;
  /**
   * Defines which step in a flow a user is in.
   */
  currentStep: number;
  setCurrentStep(step: number): void;
  /**
   * @param numStepsBack defaults to 1
   */
  prevStep(numStepsBack?: number): void;
  /**
   * @param numStepsForward defaults to 1
   */
  nextStep(numStepsForward?: number): void;
  /**
   * used when restarting the flow
   * e.g. user clicks back to the flow selection view or has finished flow
   * and wants to add another access list.
   */
  reset(): void;

  /**
   * Contains the role conditions and funcs specific to AWS IC application.
   */
  awsIcRoleState: AwsIcRoleState;
  /**
   * Contains the role conditions and funcs related to standard resources
   * e.g. server, apps (non aws ic apps), db, desktops, kube, etc.
   *
   * Standard refers to "non-specialized" resources. Specialized resource
   * example is AWS IC applications which is managed by awsIcRoleState.
   */
  standardRoleState: StandardRoleState;

  /**
   * Returns true if specified field is defined.
   */
  definedAccess(field: DefinableResourceAccessFields): boolean;
  /**
   * Returns true if an access was defined in any role conditions.
   */
  definedAccessInAnyRoleCondition(): boolean;
};

/**
 * An editor for multi step access list guides.
 *
 * Depending on the preset:
 * - No preset just handles state for current step and navigating between steps
 * - With preset also handles state for role resources intended for access list
 *
 * Other access list states like owners/members/metadata is handled differently
 * and elsewhere:
 * - When creating new access list, useCreateAccessList hook
 * - When updating, each field is individually updatable when viewing the
 *   access list
 */
export function useGuideEditor(): GuideEditorState {
  const [preset, setPreset] = useState<AccessListPreset>();
  const [currentStep, setCurrentStep] = useState(0);

  const awsIcRoleState = useAwsIcRoleState();
  const standardRoleState = useStandardRoleState();

  function reset() {
    setPreset(null);
    setCurrentStep(0);

    awsIcRoleState.reset();
    standardRoleState.reset();
  }

  function prevStep(numStepsBack = 1) {
    // Clicked "back" to the selection view.
    if (currentStep - 1 < 0) {
      reset();
      return;
    }
    setCurrentStep(currentStep - numStepsBack);
  }

  function nextStep(numStepsForward = 1) {
    setCurrentStep(currentStep + numStepsForward);
  }

  function definedAccess(resourceKind: DefinableResourceAccessFields) {
    switch (resourceKind) {
      case 'awsIc':
        return awsIcRoleState.definedAccess();

      case 'github_permissions':
      case 'app_labels':
      case 'db_labels':
      case 'kubernetes_labels':
      case 'node_labels':
      case 'windows_desktop_labels':
        return standardRoleState.definedAccess(resourceKind);
      default:
        resourceKind satisfies never;
    }
  }

  function definedAccessInAnyRoleCondition() {
    return definableResourceAccessFields.some(resourceKind =>
      definedAccess(resourceKind)
    );
  }

  return {
    reset,
    preset,
    setPreset,
    currentStep,
    setCurrentStep,
    prevStep,
    nextStep,

    awsIcRoleState,
    standardRoleState,

    definedAccess,
    definedAccessInAnyRoleCondition,
  };
}

export function isGuideEditorSupported(preset: AccessListPreset) {
  switch (preset) {
    case 'long-term':
    case 'short-term':
      return true;
    case '':
      return false;
    default:
      preset satisfies never;
  }
  return false;
}
