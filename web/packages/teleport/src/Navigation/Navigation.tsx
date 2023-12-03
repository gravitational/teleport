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

import React, { useCallback, useEffect, useState } from 'react';
import styled, { useTheme } from 'styled-components';
import { matchPath, useHistory, useLocation } from 'react-router';
import { Image } from 'design';

import { NavigationSwitcher } from 'teleport/Navigation/NavigationSwitcher';
import cfg from 'teleport/config';
import {
  NAVIGATION_CATEGORIES,
  NavigationCategory,
} from 'teleport/Navigation/categories';
import { useFeatures } from 'teleport/FeaturesContext';
import { NavigationCategoryContainer } from 'teleport/Navigation/NavigationCategoryContainer';
import { NotificationKind } from 'teleport/stores/storeNotifications';

import { useTeleport } from '..';

import logoLight from './logoLight.svg';
import logoDark from './logoDark.svg';
import logoPoweredBy from './logoPoweredBy.svg';

import type * as history from 'history';

import type { TeleportFeature } from 'teleport/types';

const NavigationContainer = styled.div`
  background: ${p => p.theme.colors.levels.surface};
  width: var(--sidebar-width);
  position: relative;
  display: flex;
  flex-direction: column;
  box-shadow: 0px 2px 1px -1px rgba(0, 0, 0, 0.2),
    0px 1px 1px rgba(0, 0, 0, 0.14), 0px 1px 3px rgba(0, 0, 0, 0.12);
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

export function Navigation({
  CustomLogo,
  showPoweredByLogo = false,
}: NavigationProps) {
  const features = useFeatures();
  const history = useHistory();
  const location = useLocation();
  const ctx = useTeleport();

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
      {CustomLogo ? <CustomLogo /> : <NavigationLogo />}

      {ctx.getFeatureFlags().managementSection && (
        <NavigationSwitcher
          onChange={handleCategoryChange}
          value={view}
          items={[
            { category: NavigationCategory.Resources },
            {
              category: NavigationCategory.Management,
              requiresAttention: ctx.storeNotifications.hasNotificationsByKind(
                NotificationKind.AccessList
              ),
            },
          ]}
        />
      )}

      <CategoriesContainer>{categories}</CategoriesContainer>
      {showPoweredByLogo && <PoweredByLogo />}
    </NavigationContainer>
  );
}

const NavigationLogo = () => {
  const theme = useTheme();

  return (
    <Image
      src={theme.type === 'dark' ? logoDark : logoLight}
      height="32px"
      width="fit-content"
      style={{
        marginTop: '20px',
        marginLeft: '32px',
        marginBottom: '20px',
      }}
      alt="teleport logo"
    />
  );
};

const PoweredByLogo = () => {
  return (
    <Image
      src={logoPoweredBy}
      height="48px"
      width="fit-content"
      style={{
        marginTop: '28px',
        marginLeft: '32px',
        marginBottom: '36px',
      }}
      alt="powered by teleport"
    />
  );
};

export type NavigationProps = {
  CustomLogo?: () => React.ReactElement;
  showPoweredByLogo?: boolean;
};
