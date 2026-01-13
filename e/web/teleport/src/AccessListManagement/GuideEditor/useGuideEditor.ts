import { useState } from 'react';

import { AccessListPreset } from 'e-teleport/services/accessmanagement/preset';

import {
  AwsIcRoleState,
  useAwsIcRoleState,
} from './Preset/DefineAccess/AwsIc/useAwsIcRoleState';

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

  function reset() {
    setPreset(null);
    setCurrentStep(0);
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

  return {
    reset,
    preset,
    setPreset,
    currentStep,
    setCurrentStep,
    prevStep,
    nextStep,

    awsIcRoleState,
  };
}
