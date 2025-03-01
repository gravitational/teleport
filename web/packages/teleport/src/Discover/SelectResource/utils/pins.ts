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
import { UserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/userpreferences_pb';

import { DiscoverGuideId } from 'teleport/services/userPreferences/discoverPreference';

import { SelectResourceSpec } from '../resources';

export const defaultPins = [
  DiscoverGuideId.ConnectMyComputer,
  DiscoverGuideId.Kubernetes,
  DiscoverGuideId.ApplicationWebHttpProxy,
  // TODO(kimlisa): add linux server, once they are all consolidated into one
];

/**
 * Only returns the defaults of resources that is available to the user.
 */
export function getDefaultPins(availableResources: SelectResourceSpec[]) {
  return availableResources
    .filter(r => defaultPins.includes(r.id))
    .map(r => r.id);
}

/**
 * Returns pins from preference if any or default pins.
 */
export function getPins(preferences: DiscoverResourcePreference) {
  if (!preferences.discoverResourcePreferences) {
    return [];
  }

  if (!preferences.discoverResourcePreferences.discoverGuide) {
    return defaultPins;
  }

  return preferences.discoverResourcePreferences.discoverGuide.pinned || [];
}

export type DiscoverResourcePreference = Partial<
  Pick<UserPreferences, 'discoverResourcePreferences'>
>;
