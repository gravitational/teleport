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
import Flex from 'design/Flex';
import Text from 'design/Text';
import {
  PendingListItem,
  RequestCheckoutProps,
} from 'shared/components/AccessRequests/NewRequest';
import {
  AccessRequest,
  LongTermResourceGrouping,
  RequestKind,
} from 'shared/services/accessRequests';

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
      <Alert kind="danger" wrapContents>
        <Flex flexDirection="column" gap={1}>
          <Text>Long-term access unavailable</Text>
          <Text typography="body2" bold={false}>
            {grouping.validationMessage ||
              'No resources are available for long-term access.'}
          </Text>
        </Flex>
      </Alert>
    );
  }

  return (
    <>
      <UncoveredResourcesError
        grouping={grouping}
        pendingAccessRequests={pendingAccessRequests}
        toggleResources={toggleResources}
      />
      <IncompatibleResourcesError
        grouping={grouping}
        pendingAccessRequests={pendingAccessRequests}
        toggleResources={toggleResources}
      />
    </>
  );
};

const UncoveredResourcesError = <T extends PendingListItem = PendingListItem>({
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
  const message = `${joinResourceNames(uncoveredResources)} ${plural ? 'are' : 'is'} not available for long-term access. Remove ${plural ? 'them' : 'it'} or switch to a short-term request.`;

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
      wrapContents
    >
      <Flex flexDirection="column" gap={1}>
        <Text>
          Long-term access is not available for{' '}
          {plural ? 'some selected resources' : 'a selected resource'}
        </Text>
        <Text typography="body2" bold={false}>
          {message}
        </Text>
      </Flex>
    </Alert>
  );
};

const IncompatibleResourcesError = <
  T extends PendingListItem = PendingListItem,
>({
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
  const message = `Remove ${joinResourceNames(incompatibleResources)} and request ${plural ? 'them' : 'it'} separately, or switch to a short-term request.`;

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
      wrapContents
    >
      <Flex flexDirection="column" gap={1}>
        <Text>
          {plural ? 'Resources' : 'Resource'} cannot be grouped for long-term
          access
        </Text>
        <Text typography="body2" bold={false}>
          {message}
        </Text>
      </Flex>
    </Alert>
  );
};

// joinResourceNames takes a list of resources and returns a
// human-readable string of their names, formatted
// as a list with commas and "and" for the last item.
const joinResourceNames = <T extends PendingListItem = PendingListItem>(
  resources: T[]
) => {
  const names = resources.map(r => r.name);
  if (names.length <= 1) return names[0] || '';
  if (names.length === 2) return names.join(' and ');
  return `${names.slice(0, -1).join(', ')} and ${names[names.length - 1]}`;
};

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
    grouping.accessListToResources?.[grouping.recommendedAccessList];
  if (!optimalGrouping?.length) {
    return [];
  }

  // Don't include uncovered resources, so we avoid duplicate errs
  const uncoveredResources = findUncoveredLongTermResources(
    grouping,
    pendingRequests
  );

  return pendingRequests.filter(
    item =>
      item.kind !== 'namespace' &&
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
      item.kind !== 'namespace' && !groupings.some(i => item.id === i.name)
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
    !dryRunResponse ||
    dryRunResponse.requestKind !== requestKind
  ) {
    return false;
  }

  const responseResourceKeys = new Set(
    dryRunResponse.resources.map(res => `${res.id.kind}:${res.id.name}`)
  );
  return pendingAccessRequests.every(r => {
    if (r.kind === 'namespace') return true;
    const key = `${r.kind}:${r.id}`;
    return responseResourceKeys.has(key);
  });
};
