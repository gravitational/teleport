/*
Copyright 2019 Gravitational, Inc.

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

import React, {
  ReactNode,
  Suspense,
  useEffect,
  useMemo,
  useState,
} from 'react';
import styled from 'styled-components';
import { Indicator } from 'design';
import { Failed } from 'design/CardError';

import useAttempt from 'shared/hooks/useAttemptNext';

import { matchPath, useHistory } from 'react-router';

import { Redirect, Route, Switch } from 'teleport/components/Router';
import { CatchError } from 'teleport/components/CatchError';
import cfg from 'teleport/config';
import useTeleport from 'teleport/useTeleport';
import { TopBar } from 'teleport/TopBar';
import { BannerList } from 'teleport/components/BannerList';
import localStorage from 'teleport/services/localStorage';

import { ClusterAlert, LINK_LABEL } from 'teleport/services/alerts/alerts';

import { Navigation } from 'teleport/Navigation';

import { useAlerts } from 'teleport/components/BannerList/useAlerts';

import { FeaturesContextProvider, useFeatures } from 'teleport/FeaturesContext';

import { getFirstRouteForCategory } from 'teleport/Navigation/Navigation';

import { NavigationCategory } from 'teleport/Navigation/categories';

import { useLayout } from 'teleport/Main/LayoutContext';

import { MainContainer } from './MainContainer';
import { OnboardDiscover } from './OnboardDiscover';

import type { BannerType } from 'teleport/components/BannerList/BannerList';
import type { LockedFeatures, TeleportFeature } from 'teleport/types';

interface MainProps {
  initialAlerts?: ClusterAlert[];
  customBanners?: ReactNode[];
  features: TeleportFeature[];
  billingBanners?: ReactNode[];
}

export function Main(props: MainProps) {
  const ctx = useTeleport();
  const history = useHistory();

  const { hasDockedElement } = useLayout();

  const { attempt, setAttempt, run } = useAttempt('processing');

  useEffect(() => {
    if (ctx.storeUser.state) {
      setAttempt({ status: 'success' });
      return;
    }

    run(() => ctx.init());
  }, []);

  const featureFlags = ctx.getFeatureFlags();

  const features = useMemo(
    () => props.features.filter(feature => feature.hasAccess(featureFlags)),
    [featureFlags, props.features]
  );

  const { alerts, dismissAlert } = useAlerts(props.initialAlerts);

  const [showOnboardDiscover, setShowOnboardDiscover] = useState(true);

  if (attempt.status === 'failed') {
    return <Failed message={attempt.statusText} />;
  }

  if (attempt.status !== 'success') {
    return (
      <StyledIndicator>
        <Indicator />
      </StyledIndicator>
    );
  }

  function handleOnboard() {
    updateOnboardDiscover();
    history.push(cfg.routes.discover);
  }

  function handleOnClose() {
    updateOnboardDiscover();
    setShowOnboardDiscover(false);
  }

  function updateOnboardDiscover() {
    const discover = localStorage.getOnboardDiscover();
    localStorage.setOnboardDiscover({ ...discover, notified: true });
  }

  // redirect to the default feature when hitting the root /web URL
  if (
    matchPath(history.location.pathname, { path: cfg.routes.root, exact: true })
  ) {
    const indexRoute = getFirstRouteForCategory(
      features,
      NavigationCategory.Resources
    );

    return <Redirect to={indexRoute} />;
  }

  // The backend defines the severity as an integer value with the current
  // pre-defined values: LOW: 0; MEDIUM: 5; HIGH: 10
  const mapSeverity = (severity: number) => {
    if (severity < 5) {
      return 'info';
    }
    if (severity < 10) {
      return 'warning';
    }
    return 'danger';
  };

  const banners: BannerType[] = alerts.map(alert => ({
    message: alert.spec.message,
    severity: mapSeverity(alert.spec.severity),
    link: alert.metadata.labels[LINK_LABEL],
    id: alert.metadata.name,
  }));

  const onboard = localStorage.getOnboardDiscover();
  const requiresOnboarding =
    onboard && !onboard.hasResource && !onboard.notified;

  return (
    <FeaturesContextProvider value={features}>
      <BannerList
        banners={banners}
        customBanners={props.customBanners}
        billingBanners={featureFlags.billing && props.billingBanners}
        onBannerDismiss={dismissAlert}
      >
        <MainContainer hasDockedElement={hasDockedElement}>
          <Navigation />
          <HorizontalSplit>
            <ContentMinWidth>
              <Suspense fallback={null}>
                <TopBar />
                <FeatureRoutes lockedFeatures={ctx.lockedFeatures} />
              </Suspense>
            </ContentMinWidth>
          </HorizontalSplit>
        </MainContainer>
      </BannerList>
      {requiresOnboarding && showOnboardDiscover && (
        <OnboardDiscover onClose={handleOnClose} onOnboard={handleOnboard} />
      )}
    </FeaturesContextProvider>
  );
}

function renderRoutes(
  features: TeleportFeature[],
  lockedFeatures: LockedFeatures
) {
  const routes = [];

  for (const [index, feature] of features.entries()) {
    const isParentLocked =
      feature.parent && new feature.parent().isLocked?.(lockedFeatures);

    // remove features with parents locked.
    // The parent itself will be rendered if it has a lockedRoute,
    // but the children shouldn't be.
    if (isParentLocked) {
      continue;
    }

    // add the route of the 'locked' variants of the features
    if (feature.isLocked?.(lockedFeatures)) {
      if (!feature.lockedRoute) {
        throw new Error('a locked feature without a locked route was found');
      }

      const { path, title, exact, component: Component } = feature.lockedRoute;
      routes.push(
        <Route title={title} key={index} path={path} exact={exact}>
          <CatchError>
            <Suspense fallback={null}>
              <Component />
            </Suspense>
          </CatchError>
        </Route>
      );

      // return early so we don't add the original route
      continue;
    }

    // add regular feature routes
    if (feature.route) {
      const { path, title, exact, component: Component } = feature.route;
      routes.push(
        <Route title={title} key={index} path={path} exact={exact}>
          <CatchError>
            <Suspense fallback={null}>
              <Component />
            </Suspense>
          </CatchError>
        </Route>
      );
    }
  }

  return routes;
}

function FeatureRoutes({ lockedFeatures }: { lockedFeatures: LockedFeatures }) {
  const features = useFeatures();
  const routes = renderRoutes(features, lockedFeatures);

  return <Switch>{routes}</Switch>;
}

export const ContentMinWidth = styled.div`
  min-width: calc(1250px - var(--sidebar-width));
`;

export const HorizontalSplit = styled.div`
  display: flex;
  flex-direction: column;
  flex: 1;
  overflow-x: auto;
`;

export const StyledIndicator = styled(HorizontalSplit)`
  align-items: center;
  justify-content: center;
`;
