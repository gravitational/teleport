import { UserEventWithData } from './types';

// AccessListStatus represents a access list wizard step outcome.
export enum AccessListStepStatusEvent {
  Success = 'ACCESS_LIST_STATUS_SUCCESS',
  Skipped = 'ACCESS_LIST_STATUS_SKIPPED',
  Error = 'ACCESS_LIST_STATUS_ERROR',
  Aborted = 'ACCESS_LIST_STATUS_ABORTED', // user exits the wizard
}

// AccessListPreset represents the access list preset type.
export enum AccessListPresetEvent {
  Unspecified = 'ACCESS_LIST_PRESET_UNSPECIFIED',
  ShortTerm = 'ACCESS_LIST_PRESET_SHORT_TERM',
  LongTerm = 'ACCESS_LIST_PRESET_LONG_TERM',
}

// AccessListIntegrate describes what integration user
// was interested in.
export enum AccessListIntegrateEvent {
  // User wants to integrate Okta or is coming from Okta.
  Okta = 'ACCESS_LIST_INTEGRATE_OKTA',
}

/**
 * AccessListEvent defines access list wizard related events.
 */
export enum AccessListEvent {
  Started = 'tp.ui.access_list.start',
  Completed = 'tp.ui.access_list.complete',
  DefineAccess = 'tp.ui.access_list.define.access',
  DefineIdentities = 'tp.ui.access_list.define.identities',
  DefineBasicInfo = 'tp.ui.access_list.define.basicinfo',
  DefineMembers = 'tp.ui.access_list.define.members',
  DefineOwners = 'tp.ui.access_list.define.owners',
  Integrate = 'tp.ui.access_list.integrate',
  Custom = 'tp.ui.access_list.custom',
}

export type AccessListStepStatus = {
  stepStatus: AccessListStepStatusEvent;
  stepStatusError?: string;
};

export type AccessListEventRequest = UserEventWithData<
  AccessListEvent,
  AccessListEventData
>;

export type AccessListEventData = AccessListStepStatus & {
  id: string;
  preset: AccessListPresetEvent;

  integrate?: AccessListIntegrateEvent;
  preferredTerraform?: boolean;
};
