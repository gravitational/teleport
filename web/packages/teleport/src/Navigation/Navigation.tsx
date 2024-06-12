/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React from 'react';
import styled from 'styled-components';
import { matchPath, useLocation, useHistory } from 'react-router';

import cfg from 'teleport/config';
import {
  NAVIGATION_CATEGORIES,
  NavigationCategory,
} from 'teleport/Navigation/categories';
import { useFeatures } from 'teleport/FeaturesContext';
import { NavigationCategoryContainer } from 'teleport/Navigation/NavigationCategoryContainer';

import type * as history from 'history';

import type { TeleportFeature } from 'teleport/types';

const NavigationContainer = styled.div`
  background: ${p => p.theme.colors.levels.surface};
  width: var(--sidebar-width);
  position: relative;
  display: flex;
  flex-direction: column;
  border-right: 1px solid ${p => p.theme.colors.spotBackground[1]};
`;

const CategoriesContainer = styled.div`
  position: relative;
  width: inherit;
  flex: 1;
`;

export function getFirstRouteForCategory(
  features: TeleportFeature[],
  category: NavigationCategory
) {
  const firstRoute = features
    .filter(feature => feature.category === category)
    .filter(feature => Boolean(feature.route))[0];

  return (
    firstRoute?.navigationItem?.getLink(cfg.proxyCluster) || cfg.routes.support
  );
}

function getFeatureForRoute(
  features: TeleportFeature[],
  route: history.Location<unknown> | Location
): TeleportFeature | undefined {
  return features.find(
    feature =>
      feature.route &&
      matchPath(route.pathname, {
        path: feature.route.path,
        exact: feature.route.exact,
      })
  );
}

function getCategoryForRoute(
  features: TeleportFeature[],
  route: history.Location<unknown> | Location
) {
  const feature = features
    .filter(feature => Boolean(feature.route))
    .find(feature =>
      matchPath(route.pathname, {
        path: feature.route.path,
        exact: false,
      })
    );

  if (!feature) {
    return;
  }

  return feature.category;
}

export function Navigation() {
  const features = useFeatures();
  const history = useHistory();
  const location = useLocation();

  const view =
    getCategoryForRoute(features, history.location) ||
    NavigationCategory.Resources;

  const categories = NAVIGATION_CATEGORIES.map((category, index) => (
    <NavigationCategoryContainer
      category={category}
      key={index}
      visible={view === category}
    />
  ));

  const feature = getFeatureForRoute(features, location);

  if (
    feature?.hideNavigation ||
    feature?.category !== NavigationCategory.Management
  ) {
    return null;
  }

  return (
    <NavigationContainer>
      <CategoriesContainer>{categories}</CategoriesContainer>
    </NavigationContainer>
  );
}
