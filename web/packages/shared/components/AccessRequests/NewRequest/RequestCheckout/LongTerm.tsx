/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
import { Alert } from 'design/Alert';
import {
  PendingListItem,
  RequestCheckoutProps,
} from 'shared/components/AccessRequests/NewRequest';
import {
  AccessRequest,
  LongTermResourceGrouping,
  RequestKind,
} from 'shared/services/accessRequests';

// UNSUPPORTED_KINDS is a list of resource_ids that are not supported
// for long-term requests.
export const UNSUPPORTED_KINDS = ['namespace', 'windows_desktop'];

// LongTermGroupingErrors displays any errors related to long-term
// resource grouping, such as uncovered or incompatible resources.
export const LongTermGroupingErrors = <
  T extends PendingListItem = PendingListItem,
>({
  grouping,
  toggleResources,
  pendingAccessRequests,
}: {
  grouping: LongTermResourceGrouping;
  toggleResources?: RequestCheckoutProps<T>['toggleResources'];
  pendingAccessRequests: T[];
}) => {
  if (
    !grouping?.accessListToResources?.[grouping?.recommendedAccessList]?.length
  ) {
    return (
      <Alert
        kind="danger"
        details={
          grouping.validationMessage ||
          'No resources are available for permanent access'
        }
        wrapContents
      >
        Permanent access unavailable
      </Alert>
    );
  }

  return (
    <>
      <LongTermUnavailableError
        grouping={grouping}
        pendingAccessRequests={pendingAccessRequests}
        toggleResources={toggleResources}
      />
      <GroupingResourcesError
        grouping={grouping}
        pendingAccessRequests={pendingAccessRequests}
        toggleResources={toggleResources}
      />
    </>
  );
};

const LongTermUnavailableError = <T extends PendingListItem = PendingListItem>({
  grouping,
  pendingAccessRequests,
  toggleResources,
}: {
  grouping: LongTermResourceGrouping;
  pendingAccessRequests: T[];
  toggleResources?: RequestCheckoutProps<T>['toggleResources'];
}) => {
  const uncoveredResources = findUncoveredLongTermResources(
    grouping,
    pendingAccessRequests
  );
  if (!uncoveredResources.length) {
    return null;
  }

  const plural = uncoveredResources.length > 1;
  const message = `${joinResourceNames(uncoveredResources)} ${plural ? 'are' : 'is'} not available for permanent access. Remove ${plural ? 'them' : 'it'} or switch to a temporary request.`;

  return (
    <Alert
      kind="danger"
      primaryAction={{
        content: `Remove incompatible ${plural ? 'resources' : 'resource'}`,
        onClick: () =>
          toggleResources(
            uncoveredResources.map(i => ({
              resourceName: i.name,
              resourceId: i.id,
              kind: i.kind,
            }))
          ),
      }}
      details={message}
      wrapContents
    >
      Permanent access is not available for{' '}
      {plural ? 'some selected resources' : 'a selected resource'}
    </Alert>
  );
};

const GroupingResourcesError = <T extends PendingListItem = PendingListItem>({
  grouping,
  pendingAccessRequests,
  toggleResources,
}: {
  grouping: LongTermResourceGrouping;
  pendingAccessRequests: T[];
  toggleResources?: RequestCheckoutProps<T>['toggleResources'];
}) => {
  const incompatibleResources = findIncompatibleLongTermResources(
    grouping,
    pendingAccessRequests
  );
  if (!incompatibleResources.length) {
    return null;
  }

  const plural = incompatibleResources.length > 1;
  const message = `Remove ${joinResourceNames(incompatibleResources)} and request ${plural ? 'them' : 'it'} separately, or switch to a temporary request.`;

  return (
    <Alert
      kind="warning"
      primaryAction={{
        content: `Remove incompatible ${plural ? 'resources' : 'resource'}`,
        onClick: () =>
          toggleResources(
            incompatibleResources.map(i => ({
              resourceName: i.name,
              resourceId: i.id,
              kind: i.kind,
            }))
          ),
      }}
      details={message}
      wrapContents
    >
      {plural ? 'Resources' : 'Resource'} cannot be grouped for permanent access
    </Alert>
  );
};

// joinResourceNames takes a list of resources and returns a
// human-readable string of their names, formatted
// as a list with commas and "and" for the last item.
const joinResourceNames = <T extends PendingListItem = PendingListItem>(
  resources: T[]
) =>
  new Intl.ListFormat('en', { style: 'long', type: 'conjunction' }).format(
    resources.map(r => (r.kind === 'namespace' ? r.subResourceName : r.name))
  );

// findIncompatibleLongTermResources iterates through the
// pendingRequests and returns a list of resources
// that are incompatible with the optimal grouping.
const findIncompatibleLongTermResources = <
  T extends PendingListItem = PendingListItem,
>(
  grouping: LongTermResourceGrouping,
  pendingRequests: T[]
) => {
  const optimalGrouping =
    grouping.accessListToResources?.[grouping.recommendedAccessList] || [];
  if (!optimalGrouping.length) {
    return [];
  }

  // Don't include uncovered resources, so we avoid duplicate errors
  const uncoveredResources = findUncoveredLongTermResources(
    grouping,
    pendingRequests
  );

  return pendingRequests.filter(
    item =>
      !uncoveredResources.some(i => item.id === i.id) &&
      !optimalGrouping.some(i => item.id === i.name)
  );
};

// findUncoveredLongTermResources iterates through the
// pendingAccessRequests and returns a list of resources
// that are not covered by any grouping.
const findUncoveredLongTermResources = <
  T extends PendingListItem = PendingListItem,
>(
  grouping: LongTermResourceGrouping,
  pendingRequests: T[]
) => {
  const groupings = Object.values(grouping.accessListToResources || {}).flat();

  return pendingRequests.filter(
    item =>
      UNSUPPORTED_KINDS.includes(item.kind) ||
      !groupings.some(i => i.name === item.id)
  );
};

// shouldShowLongTermGroupingErrors checks if the current dryRunResponse is
// up-to-date with the current pendingAccessRequests and requestKind,
// intended for use before rendering any LongTermGrouping errors.
export const shouldShowLongTermGroupingErrors = <
  T extends PendingListItem = PendingListItem,
>({
  requestKind,
  pendingAccessRequests,
  dryRunResponse,
}: {
  requestKind: RequestKind;
  pendingAccessRequests: T[];
  dryRunResponse: AccessRequest;
}) => {
  if (
    requestKind !== RequestKind.LongTerm ||
    !dryRunResponse?.longTermResourceGrouping ||
    dryRunResponse.requestKind !== requestKind
  ) {
    return false;
  }
  // If any unsupported kinds are present, show errors immediately
  if (pendingAccessRequests.some(r => UNSUPPORTED_KINDS.includes(r.kind))) {
    return true;
  }

  const responseResourceKeys = new Set(
    dryRunResponse.resources.map(res => `${res.id.kind}:${res.id.name}`)
  );
  return pendingAccessRequests.every(r => {
    const key = `${r.kind}:${r.id}`;
    return responseResourceKeys.has(key);
  });
};
