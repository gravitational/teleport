/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { useCallback, useEffect, useState } from 'react';
import styled, { useTheme } from 'styled-components';

import { matchPath, useHistory, useLocation } from 'react-router';

import { NavigationSwitcher } from 'teleport/Navigation/NavigationSwitcher';
import cfg from 'teleport/config';

import {
  NAVIGATION_CATEGORIES,
  NavigationCategory,
} from 'teleport/Navigation/categories';

import { useFeatures } from 'teleport/FeaturesContext';

import { NavigationCategoryContainer } from 'teleport/Navigation/NavigationCategoryContainer';

import logoLight from './logoLight.svg';
import logoDark from './logoDark.svg';

import type * as history from 'history';

import type { TeleportFeature } from 'teleport/types';

const NavigationLogo = styled.div`
  background: url(${props =>
      props.themeOption === 'light' ? logoLight : logoDark})
    no-repeat;
  background-size: contain;
  width: 180px;
  height: 32px;
  margin-top: 20px;
  margin-left: 32px;
  margin-bottom: 46px;
`;

const NavigationContainer = styled.div`
  background: ${p => p.theme.colors.levels.surface};
  width: var(--sidebar-width);
  position: relative;
  display: flex;
  flex-direction: column;
  box-shadow:
    0px 2px 1px -1px rgba(0, 0, 0, 0.2),
    0px 1px 1px rgba(0, 0, 0, 0.14),
    0px 1px 3px rgba(0, 0, 0, 0.12);
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
  const theme = useTheme();

  const [view, setView] = useState(
    getCategoryForRoute(features, history.location) ||
      NavigationCategory.Resources
  );

  const [previousRoute, setPreviousRoute] = useState<{
    [category: string]: string;
  }>({});

  const handleLocationChange = useCallback(
    (next: history.Location<unknown> | Location) => {
      const previousPathName = location.pathname;

      const category = getCategoryForRoute(features, next);
      const previousCategory = getCategoryForRoute(features, location);

      if (category && category !== view) {
        setView(category);

        if (previousCategory) {
          setPreviousRoute(previous => ({
            ...previous,
            [previousCategory]: previousPathName,
          }));
        }
      }
    },
    [location, view]
  );

  useEffect(() => {
    return history.listen(handleLocationChange);
  }, [history, location.pathname, features, view]);

  const handlePopState = useCallback(
    (event: PopStateEvent) => {
      handleLocationChange((event.currentTarget as Window).location);
    },
    [view]
  );

  useEffect(() => {
    window.addEventListener('popstate', handlePopState);

    return () => window.removeEventListener('popstate', handlePopState);
  }, [handlePopState]);

  const handleCategoryChange = useCallback(
    (category: NavigationCategory) => {
      if (view === category) {
        return;
      }

      history.push(
        previousRoute[category] || getFirstRouteForCategory(features, category)
      );
    },
    [view, previousRoute]
  );

  const categories = NAVIGATION_CATEGORIES.map((category, index) => (
    <NavigationCategoryContainer
      category={category}
      key={index}
      visible={view === category}
    />
  ));

  return (
    <NavigationContainer>
      <NavigationLogo themeOption={theme.name} />

      <NavigationSwitcher
        onChange={handleCategoryChange}
        value={view}
        items={NAVIGATION_CATEGORIES}
      />

      <CategoriesContainer>{categories}</CategoriesContainer>
    </NavigationContainer>
  );
}
