import { useMutation } from '@tanstack/react-query';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useLocation, useNavigate, type Location } from 'react-router';
import { useDebounceCallback } from 'usehooks-ts';

import { usePrevious } from 'shared/hooks/usePrevious';

import cfg from 'e-teleport/config';
import {
  AccessListDescriptor,
  AccessListType,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';
import {
  AccessListPreset,
  GenerateTerraformConfigRequest,
} from 'e-teleport/services/accessmanagement/preset';
import { Role, RoleVersion } from 'teleport/services/resources';
import { userEventService } from 'teleport/services/userEvent';
import {
  AccessListEvent,
  AccessListStepStatusEvent,
} from 'teleport/services/userEvent/accessListEvents';

import { customViews } from '../CreateAccessList/Guides/Custom/Custom';
import { presetGuideViews } from '../CreateAccessList/Guides/Preset/Presets';
import { AccessListView } from '../CreateAccessList/Guides/Preset/view';
import {
  ResumableGuideEditorState,
  ResumableGuideState,
} from '../CreateAccessList/route';
import { AccessListEmitEvent, getAccessListPresetForEvent } from './event';
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
import { defaultSidePanelWidth } from './Shared';
import { hasTerraformLabel } from './Terraform/terraform';

/**
 * Do not change.
 *
 * This is the lowest role version that this guide editor supports.
 * It also marks the start of this guide editor feature.
 */
export const MinimumRoleVersionSupported = RoleVersion.V8;

type DeploymentView = '' | 'finished' | 'terraform';

export type Terraform = {
  config: string;
  /**
   * Used to always have the latest config rendered in the UI
   * while the new config is being generated.
   */
  prevConfig: string;
  mutatePending: boolean;
  mutateError: Error | null;
  regenerateConfig: (req: GenerateTerraformConfigRequest) => void;
  /**
   * True once regenerateConfig has been called at least once.
   * Used to determine the initial empty state to skip calling
   * the API and render helpful terraform config text for user.
   */
  hasMutatedConfig: boolean;
  /**
   * If zero, terraform panel will not be visible.
   * Otherwise, it represents the width of the panel in pixels.
   */
  sidePanel: number;
  updateSidePanel: (width: number) => void;
};

export type GuideEditorState = {
  /**
   * Describes the type of selected preset.
   * Will be initially null to mean "no preset selected".
   */
  preset: AccessListPreset;
  setPreset(kind: AccessListPreset): void;
  onGuideSelect(kind: AccessListPreset): void;
  views: AccessListView[];

  /**
   * Holds state and actions for generating and displaying Terraform config
   */
  terraform: Terraform;

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
   * Tracks the current view within the DeploymentMethods step.
   * Used to gate the abort event so it only fires when the user
   * leaves without selecting a deployment method.
   */
  deploymentView: DeploymentView;
  setDeploymentView(view: DeploymentView): void;

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

  /**
   * Discards all unsaved role edits by re-initializing role states with
   * their original values and resets the current step.
   *
   * Used when user "cancels/closes" the editor.
   */
  undoEditRoleChanges(): void;
  /**
   * True when editing existing roles, false when creating new ones.
   */
  isEditing: boolean;
  /**
   * Returns a list of roles to update.
   */
  getRolesToSave(): Role[];
  /**
   * Returns the current editor state that can be passed to location state.
   * Used to support resuming a flow when navigating away from guide and
   * coming back to it (e.g: enroll okta integration -> resume guide).
   */
  getResumableState(): ResumableGuideState;
  /**
   * True if user came from okta integration.
   */
  originatedFromOkta: boolean;
  /**
   * Removes location state.
   */
  removeLocationState(): void;

  /**
   * Emits usage event for product metrics.
   *
   * Only emits event when creating.
   */
  emitEvent(event: AccessListEmitEvent): void;
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
  const navigate = useNavigate();
  const loc = useLocation() as Location<ResumableGuideEditorState>;

  const [preset, setPreset] = useState<AccessListPreset>(
    () => loc.state?.preset ?? null
  );
  const [currentStep, setCurrentStep] = useState(
    () => loc.state?.resumeStep ?? 0
  );

  const [eventSessionId, setEventSessionId] = useState(
    () => loc.state?.eventSessionId ?? crypto.randomUUID()
  );

  const [deploymentView, setDeploymentView] = useState<DeploymentView>('');

  const [terraformPanel, setTerraformPanel] = useState(defaultSidePanelWidth);

  const refAbortCtrl = useRef<AbortController>(null);
  const terraformConfigMutation = useMutation({
    mutationFn: (req: GenerateTerraformConfigRequest) => {
      refAbortCtrl.current?.abort();
      refAbortCtrl.current = new AbortController();
      return accessManagementService.generateTerraformConfig(
        req,
        refAbortCtrl.current.signal
      );
    },
    retry: (failureCount, error) => {
      if (error.name === 'AbortError') {
        return false;
      }
      return failureCount < 3;
    },
    scope: {
      id: 'accesslist-preset-terraform',
    },
  });
  const prevTerraformConfig = usePrevious(terraformConfigMutation?.data);
  const hasMutatedTerraformConfigRef = useRef(false);
  const debouncedTerraformMutate = useDebounceCallback(
    terraformConfigMutation.mutate,
    1000
  );

  function regenerateTerraformConfig(req: GenerateTerraformConfigRequest) {
    hasMutatedTerraformConfigRef.current = true;
    debouncedTerraformMutate(req);
  }

  const terraform: Terraform = {
    config: terraformConfigMutation.data?.terraform || '',
    prevConfig: prevTerraformConfig?.terraform || '',
    regenerateConfig: regenerateTerraformConfig,
    mutateError: terraformConfigMutation.error,
    mutatePending: terraformConfigMutation.isPending,
    hasMutatedConfig: hasMutatedTerraformConfigRef.current,
    sidePanel: terraformPanel,
    updateSidePanel: setTerraformPanel,
  };

  const views: AccessListView[] = useMemo(() => {
    switch (preset) {
      case 'long-term':
      case 'short-term':
        return presetGuideViews;
      case '':
        return customViews;
      default:
        preset satisfies never;
        return [];
    }
  }, [preset]);

  const awsIcRoleState = useAwsIcRoleState();
  const standardRoleState = useStandardRoleState();

  const isEditing =
    !!standardRoleState.roleEditState || !!awsIcRoleState.roleEditState;

  function reset() {
    setDeploymentView('');
    setPreset(null);
    setCurrentStep(0);
    setEventSessionId(crypto.randomUUID());

    awsIcRoleState.reset();
    standardRoleState.reset();

    resetTerraform();

    removeLocationState();
  }

  function resetTerraform() {
    // Stops debounce timer.
    debouncedTerraformMutate.cancel();
    // Cancels any in-flight HTTP request.
    refAbortCtrl.current?.abort();
    terraformConfigMutation.reset();
    hasMutatedTerraformConfigRef.current = false;
    setTerraformPanel(defaultSidePanelWidth);
  }

  function removeLocationState() {
    if (loc.state) {
      navigate(loc.pathname, { replace: true });
    }
  }

  function emitEvent({
    event,
    stepStatus,
    integrate,
    stepStatusError,
    preferredTerraform,
  }: AccessListEmitEvent) {
    // Only emit events when creating.
    if (isEditing || !views.length) return;

    const currView = views[currentStep];

    userEventService.captureAccessListEvent({
      event: event ?? currView.eventName,
      eventData: {
        id: eventSessionId,
        stepStatus,
        integrate,
        preset: getAccessListPresetForEvent(preset),
        stepStatusError,
        preferredTerraform,
      },
    });
  }

  // Only support emitting events when creating new access lists.
  async function emitAbortEvent() {
    if (views.length && !isEditing) {
      const curView = views[currentStep];
      if (curView.eventName) {
        if (
          !curView.isFinishedStep ||
          (curView.isFinishedStep && deploymentView === '')
        ) {
          emitEvent({
            event: curView.eventName,
            stepStatus: AccessListStepStatusEvent.Aborted,
          });
        }
      }
    }
  }

  function onGuideSelect(preset: AccessListPreset) {
    setPreset(preset);
    userEventService.captureAccessListEvent({
      event: AccessListEvent.Started,
      eventData: {
        id: eventSessionId,
        stepStatus: AccessListStepStatusEvent.Success,
        preset: getAccessListPresetForEvent(preset),
      },
    });
  }

  // Keeping a ref to the latest emitAbortEvent func to ensure the
  // correct event is emitted. It also avoids using it as a dependency
  // in the useEffect which would cause it to re-run and add/remove
  // event listeners on every render.
  const emitAbortEventRef = useRef(emitAbortEvent);
  emitAbortEventRef.current = emitAbortEvent;

  // Emit abort event upon refreshing, navigating to a different route,
  // closing tab/browser, or unmounting.
  //
  // "beforeunload" is used for close/refresh — there can be a false positive
  // if the user cancels the default browser reload dialog.
  useEffect(() => {
    let emittingAbortEvent = false;
    const emitEvent = () => {
      if (!emittingAbortEvent) {
        emittingAbortEvent = true;
        emitAbortEventRef.current();
      }
    };

    window.addEventListener('beforeunload', emitEvent);
    return () => {
      window.removeEventListener('beforeunload', emitEvent);
      // Don't emit abort if navigating to okta — user may come back
      // to finish the flow.
      const navigatedToOkta = window.location.pathname.startsWith(
        cfg.oss.getIntegrationEnrollRoute('okta')
      );
      if (!navigatedToOkta) {
        emitEvent();
      }
    };
  }, []);

  function undoEditRoleChanges() {
    setCurrentStep(0);
    resetTerraform();

    if (standardRoleState.roleEditState) {
      standardRoleState.initRoleEditState(
        standardRoleState.roleEditState.original
      );
    }

    if (awsIcRoleState.roleEditState) {
      awsIcRoleState.initRoleEditState(awsIcRoleState.roleEditState.original);
    }
  }

  function prevStep(numStepsBack = 1) {
    // Clicked "back" to the selection view.
    if (currentStep - 1 < 0) {
      emitAbortEvent();
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
      case 'linux_desktop_labels':
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

  function getRolesToSave() {
    const accessRoles: Role[] = [];

    const awsIcRole = awsIcRoleState.getRoleToSave();
    const standardRole = standardRoleState.getRoleToSave();

    if (awsIcRole) {
      accessRoles.push(awsIcRole);
    }

    if (standardRole) {
      accessRoles.push(standardRole);
    }

    return accessRoles;
  }

  function getResumableState(): ResumableGuideState {
    return {
      preset,
      resumeStep: currentStep,
      standardRoleConditions: standardRoleState.roleConditions,
      requiredAppIdentities: standardRoleState.requiredAppIdentities,
      awsIcRoleConditions: awsIcRoleState.roleConditions,
      oktaOrgUrl: '',
      eventSessionId,
    };
  }

  return {
    reset,
    preset,
    setPreset,
    terraform,
    onGuideSelect,
    views,
    currentStep,
    setCurrentStep,
    prevStep,
    nextStep,
    originatedFromOkta: !!loc.state?.oktaOrgUrl,
    removeLocationState,
    deploymentView,
    setDeploymentView,

    emitEvent,

    awsIcRoleState,
    standardRoleState,

    definedAccess,
    definedAccessInAnyRoleCondition,

    getRolesToSave,

    getResumableState,

    undoEditRoleChanges,
    isEditing,
  };
}

function hasSupportedPreset(preset: AccessListPreset) {
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

export function isGuideEditorSupported(accessList: AccessListDescriptor) {
  if (accessList.type === AccessListType.Static) {
    return (
      hasTerraformLabel(accessList) && hasSupportedPreset(accessList.preset)
    );
  }

  if (accessList.type === AccessListType.Default) {
    return hasSupportedPreset(accessList.preset);
  }

  return false;
}
