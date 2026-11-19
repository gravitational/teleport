/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { useState } from 'react';

import {
  getPendingRequestDurationOptions,
  ReviewerOption,
} from 'shared/components/AccessRequests/NewRequest';
import { Option } from 'shared/components/Select';
import { AccessRequest, RequestKind } from 'shared/services/accessRequests';

import {
  getDurationOptionIndexClosestToHours,
  getDurationOptionIndexClosestToOneWeek,
  getDurationOptionsFromStartTime,
} from '../AccessDuration/durationOptions';
import { makeAccessEndOption } from './RequestCheckout/utils';

export function useSpecifiableFields() {
  /**
   * Fetched response from `createAccessRequest` with dry run enabled.
   * Contains max time options (to calculate max duration and requestTTL options)
   * and suggested reviewers that were available both statically (from roles)
   * and dynamically (from access lists).
   */
  const [dryRunResponse, setDryRunResponse] = useState<AccessRequest>();

  /**
   * Fetched response from `fetchResourceRequestRoles`.
   * Required roles to gain access to the resources requested.
   */
  const [resourceRequestRoles, setResourceRequestRoles] = useState<string[]>(
    []
  );

  const [selectedResourceRequestRoles, setSelectedResourceRequestRoles] =
    useState<string[]>([]);

  /**
   * User selected reviewers from suggested reviewers options and/or
   * any other reviewers they manually added.
   */
  const [selectedReviewers, setSelectedReviewers] = useState<ReviewerOption[]>(
    []
  );

  /**
   * Specifies when the access request can be "assumed" by.
   * No startTime means access request is to be available
   * to assume immediately.
   */
  const [startTime, setStartTime] = useState<Date | null>();

  // The public checkout contract calls this maxDuration, but the option value
  // is the selected absolute access end.
  const [selectedAccessEnd, setSelectedAccessEnd] = useState<Option<number>>();

  /**
   * How long the request can be in a PENDING state before it expires.
   * Resets when startTime changes.
   */
  const [pendingRequestTtl, setPendingRequestTtl] = useState<Option<number>>();

  /**
   * Options for shortening or extending pending TTL.
   */
  let pendingRequestTtlOptions: Option<number>[] = [];
  /**
   * Options for extending or shortening the access request duration.
   */
  let accessDurationOptions: Option<number>[] = [];

  const [requestKind, setRequestKind] = useState<RequestKind>(
    RequestKind.ShortTerm
  );

  if (dryRunResponse) {
    pendingRequestTtlOptions = getPendingRequestDurationOptions(
      dryRunResponse.created,
      selectedAccessEnd.value
    );
    accessDurationOptions = getDurationOptionsFromStartTime(
      startTime,
      dryRunResponse
    );
    // Keep the user's selected duration in the list when they switch back to
    // "Immediately".
    if (
      dryRunResponse.timing?.scheduled &&
      selectedAccessEnd &&
      !accessDurationOptions.some(
        option => option.value === selectedAccessEnd.value
      )
    ) {
      accessDurationOptions = [
        ...accessDurationOptions,
        selectedAccessEnd,
      ].sort((left, right) => left.value - right.value);
    }
  }

  function reset() {
    setDryRunResponse(null);
    setResourceRequestRoles([]);
    setSelectedResourceRequestRoles([]);
    setSelectedReviewers([]);
    setStartTime(null);
    setSelectedAccessEnd(null);
    setPendingRequestTtl(null);
    setRequestKind(RequestKind.ShortTerm);
  }

  function preselectPendingRequestTtlOption(
    accessEnd: number,
    dryRequestCreated: Date
  ) {
    const newRequestTtlOptions = getPendingRequestDurationOptions(
      dryRequestCreated,
      accessEnd
    );
    const ttlIndex = getDurationOptionIndexClosestToOneWeek(
      newRequestTtlOptions,
      dryRequestCreated
    );
    setPendingRequestTtl(newRequestTtlOptions[ttlIndex]);
  }

  function onMaxDurationChange(newAccessEndOption: Option<number>) {
    setSelectedAccessEnd(newAccessEndOption);
    // Re-set pending request TTL, since pending duration
    // can't be greater than max duration.
    preselectPendingRequestTtlOption(
      newAccessEndOption.value,
      dryRunResponse.created
    );
  }

  /**
   * An empty start date will default to dry run's response created date.
   */
  function onStartTimeChange(start?: Date) {
    updateStartAndDurationFields(start, dryRunResponse, selectedAccessEnd);
  }

  function updateStartAndDurationFields(
    start: Date,
    dryRequest: AccessRequest,
    accessEndToPreserve?: Option<number> | null
  ) {
    const scheduledTiming = dryRequest.timing?.scheduled;
    const selectedStart = start ?? scheduledTiming?.start ?? dryRequest.created;
    let accessEndOption: Option<number>;

    if (scheduledTiming && accessEndToPreserve) {
      // Keep the user's duration unchanged when the start changes.
      const previousStart = getSelectedStart(dryRequest, startTime);
      const selectedDuration =
        accessEndToPreserve.value - previousStart.getTime();
      accessEndOption = makeAccessEndOption(
        selectedStart,
        new Date(selectedStart.getTime() + selectedDuration)
      );
    } else {
      // Use the default duration when no scheduled duration is selected.
      accessEndOption = getDefaultAccessEndOption(selectedStart, dryRequest);
    }

    setStartTime(start);
    setSelectedAccessEnd(accessEndOption);
    preselectPendingRequestTtlOption(accessEndOption.value, dryRequest.created);
  }

  /**
   * An empty dry run request will just set the response to null.
   * Teleterm requires clearing this response when user closes
   * checkout.
   */
  function onDryRunChange(dryRequest?: AccessRequest) {
    setDryRunResponse(dryRequest);

    if (dryRequest) {
      // Initialize fields.
      updateStartAndDurationFields(null /* startTime */, dryRequest);

      // Initially select suggested reviewers for the requestor.
      const reviewers = dryRequest.reviewers.map(r => r.name);
      setSelectedReviewers(
        reviewers.map(r => ({
          value: r,
          label: r,
          isSelected: true,
        }))
      );
    }
  }

  return {
    selectedReviewers,
    setSelectedReviewers,
    resourceRequestRoles,
    setResourceRequestRoles,
    selectedResourceRequestRoles,
    setSelectedResourceRequestRoles,
    maxDuration: selectedAccessEnd,
    onMaxDurationChange,
    maxDurationOptions: accessDurationOptions,
    pendingRequestTtl,
    setPendingRequestTtl,
    pendingRequestTtlOptions,
    dryRunResponse,
    startTime,
    requestKind,
    setRequestKind,
    onStartTimeChange,
    onDryRunChange,
    reset,
    reasonMode: dryRunResponse?.reasonMode,
    reasonPrompts: dryRunResponse?.reasonPrompts,
  };
}

function getDefaultAccessEndOption(
  start: Date,
  accessRequest: AccessRequest
): Option<number> {
  const options = getDurationOptionsFromStartTime(start, accessRequest);
  const index = accessRequest.timing?.scheduled
    ? getDurationOptionIndexClosestToHours(options, start, 8)
    : getDurationOptionIndexClosestToOneWeek(options, start);
  return options[index];
}

function getSelectedStart(
  accessRequest: AccessRequest,
  selectedStart: Date
): Date {
  return (
    selectedStart ||
    accessRequest.timing?.scheduled?.start ||
    accessRequest.created
  );
}
