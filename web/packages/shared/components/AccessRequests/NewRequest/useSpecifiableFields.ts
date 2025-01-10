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
import { AccessRequest } from 'shared/services/accessRequests';

import {
  getDurationOptionIndexClosestToOneWeek,
  getDurationOptionsFromStartTime,
} from '../AccessDuration/durationOptions';

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

  /**
   * Specifies how long the access request should last for.
   * Duration countdown starts from access request creation.
   * Resets when startTime changes.
   */
  const [maxDuration, setMaxDuration] = useState<Option<number>>();

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
  let maxDurationOptions: Option<number>[] = [];

  if (dryRunResponse) {
    pendingRequestTtlOptions = getPendingRequestDurationOptions(
      dryRunResponse.created,
      maxDuration.value
    );
    maxDurationOptions = getDurationOptionsFromStartTime(
      startTime,
      dryRunResponse
    );
  }

  function reset() {
    setDryRunResponse(null);
    setResourceRequestRoles([]);
    setSelectedResourceRequestRoles([]);
    setSelectedReviewers([]);
    setStartTime(null);
    setMaxDuration(null);
    setPendingRequestTtl(null);
  }

  function preselectPendingRequestTtlOption(
    newMaxDuration: number,
    dryRequestCreated: Date
  ) {
    const newRequestTtlOptions = getPendingRequestDurationOptions(
      dryRequestCreated,
      newMaxDuration
    );
    const ttlIndex = getDurationOptionIndexClosestToOneWeek(
      newRequestTtlOptions,
      dryRequestCreated
    );
    setPendingRequestTtl(newRequestTtlOptions[ttlIndex]);
  }

  function onMaxDurationChange(newMaxDurationOption: Option<number>) {
    setMaxDuration(newMaxDurationOption);
    // Re-set pending request TTL, since pending duration
    // can't be greater than max duration.
    preselectPendingRequestTtlOption(
      newMaxDurationOption.value,
      dryRunResponse.created
    );
  }

  /**
   * An empty start date will default to dry run's response created date.
   */
  function onStartTimeChange(start?: Date) {
    updateStartAndDurationFields(start, dryRunResponse);
  }

  function updateStartAndDurationFields(
    start: Date,
    dryRequest: AccessRequest
  ) {
    setStartTime(start);

    // Pre-select default max duration.
    const newDurationOpts = getDurationOptionsFromStartTime(start, dryRequest);

    const durationIndex = getDurationOptionIndexClosestToOneWeek(
      newDurationOpts,
      start || dryRequest.created
    );
    const newMaxDuration = newDurationOpts[durationIndex];
    setMaxDuration(newMaxDuration);

    preselectPendingRequestTtlOption(newMaxDuration.value, dryRequest.created);
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
    maxDuration,
    onMaxDurationChange,
    maxDurationOptions,
    pendingRequestTtl,
    setPendingRequestTtl,
    pendingRequestTtlOptions,
    dryRunResponse,
    startTime,
    onStartTimeChange,
    onDryRunChange,
    reset,
  };
}
