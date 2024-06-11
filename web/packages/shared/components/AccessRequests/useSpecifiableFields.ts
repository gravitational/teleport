import { useState } from 'react';

import { Option } from 'shared/components/Select';
import {
  getPendingRequestDurationOptions,
  ReviewerOption,
} from 'shared/components/AccessRequests/NewRequest';
import { AccessRequest } from 'shared/services/accessRequests';

import {
  getDurationOptionIndexClosestToOneWeek,
  getDurationOptionsFromStartTime,
} from './AccessDuration/durationOptions';

export function useSpecifiableFields() {
  // Fetched response from `createAccessRequest` with dry run enabled.
  // Contains max time options (to calculate max duration and requestTTL options)
  // and suggested reviewers that were available both statically (from roles)
  // and dynamically (from access lists).
  const [dryRunResponse, setDryRunResponse] = useState<AccessRequest>();

  // Fetched response from `fetchResourceRequestRoles`.
  // Required roles to gain access to the resources requested.
  const [resourceRequestRoles, setResourceRequestRoles] = useState<string[]>(
    []
  );

  const [selectedResourceRequestRoles, setSelectedResourceRequestRoles] =
    useState<string[]>([]);

  // User selected reviewers from suggested reviewers options and/or
  // any other reviewers they manually added.
  const [selectedReviewers, setSelectedReviewers] = useState<ReviewerOption[]>(
    []
  );

  // Specifies when the access request can be "assumed" by.
  // No startTime means access request is to be available
  // to assume immediately.
  const [startTime, setStartTime] = useState<Date | null>();

  // Specifies how long the access request should last for.
  // Duration countdown starts from access request creation.
  // Resets when startTime changes.
  const [maxDuration, setMaxDuration] = useState<Option<number>>();

  // How long the request can be in a PENDING state before it expires.
  // Resets when startTime changes.
  const [pendingRequestTtl, setPendingRequestTtl] = useState<Option<number>>();

  // An empty start date will default to dry run's response created date.
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

    // Pre-select default request ttl.
    const newRequestTtlOptions = getPendingRequestDurationOptions(
      dryRequest.created,
      newMaxDuration.value
    );
    const ttlIndex = getDurationOptionIndexClosestToOneWeek(
      newRequestTtlOptions,
      dryRequest.created
    );
    setPendingRequestTtl(newRequestTtlOptions[ttlIndex]);
  }

  function initSpecifiableFields(dryRequest: AccessRequest) {
    setDryRunResponse(dryRequest);

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

  return {
    selectedReviewers,
    setSelectedReviewers,
    resourceRequestRoles,
    setResourceRequestRoles,
    selectedResourceRequestRoles,
    setSelectedResourceRequestRoles,
    maxDuration,
    setMaxDuration,
    pendingRequestTtl,
    setPendingRequestTtl,
    dryRunResponse,
    setDryRunResponse,
    startTime,
    onStartTimeChange,
    initSpecifiableFields,
  };
}
