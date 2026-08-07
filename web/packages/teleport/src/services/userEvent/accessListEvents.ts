/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
