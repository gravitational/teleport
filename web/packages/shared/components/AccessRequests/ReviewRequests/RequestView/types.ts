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

import { AllUserTraits } from 'teleport/services/user';

import { RequestState } from 'shared/services/accessRequests';

export type RequestFlags = {
  /** Describes request is own request and request is approved */
  canAssume: boolean;
  /**
   * Decides if the button to assume a request should be disabled
   * and determines the text on it.
   */
  isAssumed: boolean;
  canReview: boolean;
  canDelete: boolean;
  ownRequest: boolean;
  isPromoted: boolean;
};

/** Subset of `AccessList` properties required to show a suggestion. */
export type SuggestedAccessList = {
  id: string;
  title: string;
  description?: string;
  grants: {
    roles: string[];
    traits: AllUserTraits;
  };
};

export type SubmitReview = {
  state: RequestState;
  reason: string;
  promotedToAccessList?: SuggestedAccessList;
  assumeStartTime?: Date;
};
