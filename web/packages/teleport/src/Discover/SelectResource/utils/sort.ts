/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { getPlatform } from 'design/platform';
import { Resource } from 'gen-proto-ts/teleport/userpreferences/v1/onboard_pb';
import { UserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/userpreferences_pb';

import { OnboardDiscover } from 'teleport/services/user';

import { ResourceKind } from '../../Shared';
import { resourceKindToPreferredResource } from '../../Shared/ResourceKind';
import { getMarketingTermMatches } from '../getMarketingTermMatches';
import { PrioritizedResources, ResourceSpec, SearchResource } from '../types';

function isConnectMyComputerAvailable(
  accessibleResources: ResourceSpec[]
): boolean {
  return !!accessibleResources.find(
    resource => resource.kind === ResourceKind.ConnectMyComputer
  );
}

export function sortResourcesByPreferences(
  resources: ResourceSpec[],
  preferences: UserPreferences,
  onboardDiscover: OnboardDiscover | undefined
) {
  const { preferredResources, hasPreferredResources } =
    getPrioritizedResources(preferences);
  const platform = getPlatform();

  const sortedResources = [...resources];
  const accessible = sortedResources.filter(r => r.hasAccess);
  const restricted = sortedResources.filter(r => !r.hasAccess);

  const hasNoResources = onboardDiscover && !onboardDiscover.hasResource;
  const prefersServers =
    hasPreferredResources &&
    preferredResources.includes(
      resourceKindToPreferredResource(ResourceKind.Server)
    );
  const prefersServersOrNoPreferences =
    prefersServers || !hasPreferredResources;
  const shouldShowConnectMyComputerFirst =
    hasNoResources &&
    prefersServersOrNoPreferences &&
    isConnectMyComputerAvailable(accessible);

  // Sort accessible resources by:
  // 1. os
  // 2. preferred
  // 3. guided
  // 4. alphabetically
  //
  // When available on the given platform, Connect My Computer is put either as the first resource
  // if the user has no resources, otherwise it's at the end of the guided group.
  accessible.sort((a, b) => {
    const compareAB = (predicate: (r: ResourceSpec) => boolean) =>
      comparePredicate(a, b, predicate);
    const areBothGuided = !a.unguidedLink && !b.unguidedLink;

    // Special cases for Connect My Computer.
    // Show Connect My Computer tile as the first resource.
    if (shouldShowConnectMyComputerFirst) {
      const prioritizeConnectMyComputer = compareAB(
        r => r.kind === ResourceKind.ConnectMyComputer
      );
      if (prioritizeConnectMyComputer) {
        return prioritizeConnectMyComputer;
      }

      // Within the guided group, deprioritize server tiles of the current user platform if Connect
      // My Computer is available.
      //
      // If the user has no resources available in the cluster, we want to nudge them towards
      // Connect My Computer rather than, say, standalone macOS setup.
      //
      // Only do this if the user doesn't explicitly prefer servers. If they prefer servers, we
      // want the servers for their platform to be displayed in their usual place so that the user
      // doesn't miss that Teleport supports them.
      if (!prefersServers && areBothGuided) {
        const deprioritizeServerForUserPlatform = compareAB(
          r => !(r.kind == ResourceKind.Server && r.platform === platform)
        );
        if (deprioritizeServerForUserPlatform) {
          return deprioritizeServerForUserPlatform;
        }
      }
    } else if (areBothGuided) {
      // Show Connect My Computer tile as the last guided resource if the user already added some
      // resources or they prefer other kinds of resources than servers.
      const deprioritizeConnectMyComputer = compareAB(
        r => r.kind !== ResourceKind.ConnectMyComputer
      );
      if (deprioritizeConnectMyComputer) {
        return deprioritizeConnectMyComputer;
      }
    }

    // Display platform resources first
    const prioritizeUserPlatform = compareAB(r => r.platform === platform);
    if (prioritizeUserPlatform) {
      return prioritizeUserPlatform;
    }

    // Display preferred resources second
    if (hasPreferredResources) {
      const prioritizePreferredResource = compareAB(r =>
        preferredResources.includes(resourceKindToPreferredResource(r.kind))
      );
      if (prioritizePreferredResource) {
        return prioritizePreferredResource;
      }
    }

    // Display guided resources third
    const prioritizeGuided = compareAB(r => !r.unguidedLink);
    if (prioritizeGuided) {
      return prioritizeGuided;
    }

    // Alpha
    return a.name.localeCompare(b.name);
  });

  // Sort restricted resources alphabetically
  restricted.sort((a, b) => {
    return a.name.localeCompare(b.name);
  });

  // Sort resources that user has access to the
  // top of the list, so it is more visible to
  // the user.
  return [...accessible, ...restricted];
}

/**
 * Returns prioritized resources based on user preferences cluster state
 *
 * @remarks
 * A user can have preferredResources set via onboarding either from the survey (preferredResources)
 * or various query parameters (marketingParams). We sort the list by the marketingParams if available.
 * If not, we sort by preferred resource type if available.
 * We do not search.
 *
 * @param preferences - Cluster state user preferences
 * @returns PrioritizedResources which is both the resource to prioritize and a boolean value of the value
 *
 */
function getPrioritizedResources(
  preferences: UserPreferences
): PrioritizedResources {
  const marketingParams = preferences.onboard.marketingParams;

  if (marketingParams) {
    const marketingPriorities = getMarketingTermMatches(marketingParams);
    if (marketingPriorities.length > 0) {
      return {
        hasPreferredResources: true,
        preferredResources: marketingPriorities,
      };
    }
  }

  const preferredResources = preferences.onboard.preferredResources || [];

  // hasPreferredResources will be false if all resources are selected
  const maxResources = Object.keys(Resource).length / 2 - 1;
  const selectedAll = preferredResources.length === maxResources;

  return {
    preferredResources: preferredResources,
    hasPreferredResources: preferredResources.length > 0 && !selectedAll,
  };
}

const aBeforeB = -1;
const aAfterB = 1;
const aEqualsB = 0;

/**
 * Evaluates the predicate and prioritizes the element matching the predicate over the element that
 * doesn't.
 *
 * @example
 * comparePredicate({color: 'green'}, {color: 'red'}, (el) => el.color === 'green') // => -1 (a before b)
 * comparePredicate({color: 'red'}, {color: 'green'}, (el) => el.color === 'green') // => 1  (a after  b)
 * comparePredicate({color: 'blue'}, {color: 'pink'}, (el) => el.color === 'green') // => 0  (both are equal)
 */
function comparePredicate<ElementType>(
  a: ElementType,
  b: ElementType,
  predicate: (resource: ElementType) => boolean
): -1 | 0 | 1 {
  const aMatches = predicate(a);
  const bMatches = predicate(b);

  if (aMatches && !bMatches) {
    return aBeforeB;
  }

  if (bMatches && !aMatches) {
    return aAfterB;
  }

  return aEqualsB;
}

export function sortResourcesByKind(
  resourceKind: SearchResource,
  resources: ResourceSpec[]
) {
  let sorted: ResourceSpec[] = [];
  switch (resourceKind) {
    case SearchResource.SERVER:
      sorted = [
        ...resources.filter(r => r.kind === ResourceKind.Server),
        ...resources.filter(r => r.kind !== ResourceKind.Server),
      ];
      break;
    case SearchResource.APPLICATION:
      sorted = [
        ...resources.filter(r => r.kind === ResourceKind.Application),
        ...resources.filter(r => r.kind !== ResourceKind.Application),
      ];
      break;
    case SearchResource.DATABASE:
      sorted = [
        ...resources.filter(r => r.kind === ResourceKind.Database),
        ...resources.filter(r => r.kind !== ResourceKind.Database),
      ];
      break;
    case SearchResource.DESKTOP:
      sorted = [
        ...resources.filter(r => r.kind === ResourceKind.Desktop),
        ...resources.filter(r => r.kind !== ResourceKind.Desktop),
      ];
      break;
    case SearchResource.KUBERNETES:
      sorted = [
        ...resources.filter(r => r.kind === ResourceKind.Kubernetes),
        ...resources.filter(r => r.kind !== ResourceKind.Kubernetes),
      ];
      break;
  }
  return sorted;
}
